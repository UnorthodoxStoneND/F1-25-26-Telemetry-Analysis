package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	flagPitIn uint8 = 1 << iota
	flagPitOut
	flagSC
	flagVSC
	flagGapKnown
	flagInvalid
)

type RaceObservation struct {
	Time          float64
	Frame         uint32
	Lap, Position int
	GapMS         uint32
	Flags         uint8
}

// Store phase/position changes and one-second checkpoints. Preserve the exact
// last packet at every lap/phase boundary without retaining a second full feed.
func (d *RecordedDriver) observeRace(o RaceObservation) {
	if !d.raceLastSet {
		d.raceObservations = append(d.raceObservations, o)
	} else {
		last := d.raceLast
		checkpoint := d.raceObservations[len(d.raceObservations)-1]
		if o.Lap != last.Lap || o.Flags != last.Flags || o.Position != last.Position || o.Time-checkpoint.Time >= 1 {
			if last.Frame != checkpoint.Frame {
				d.raceObservations = append(d.raceObservations, last)
			}
			d.raceObservations = append(d.raceObservations, o)
		}
	}
	d.raceLast = o
	d.raceLastSet = true
}
func (d *RecordedDriver) rewindRaceObservations(clock float64, frame *uint32) {
	valid := func(o RaceObservation) bool {
		return o.Time <= clock && (frame == nil || o.Time != clock || o.Frame <= *frame)
	}
	kept := d.raceObservations[:0]
	for _, o := range d.raceObservations {
		if valid(o) {
			kept = append(kept, o)
		}
	}
	d.raceObservations = kept
	if d.raceLastSet && !valid(d.raceLast) {
		d.raceLastSet = false
		if len(kept) > 0 {
			d.raceLast = kept[len(kept)-1]
			d.raceLastSet = true
		}
	}
}

type RaceLapMetric struct {
	Lap      int
	Time     uint32
	Sectors  [3]uint32
	Position int
	GapMS    uint32
	Flags    uint8
	Compound int
}
type RaceMetrics struct {
	Laps   [24]map[int]RaceLapMetric
	Median map[int]float64
	MaxLap int
}

func (m *AnalysisModel) metrics() *RaceMetrics {
	if m.raceMetrics != nil {
		return m.raceMetrics
	}
	out := &RaceMetrics{Median: map[int]float64{}}
	field := map[int][]float64{}
	for i := range m.Drivers {
		d := &m.Drivers[i]
		out.Laps[i] = map[int]RaceLapMetric{}
		apply := func(o RaceObservation) {
			r := out.Laps[i][o.Lap]
			r.Lap = o.Lap
			r.Position = o.Position
			r.Flags |= o.Flags &^ flagGapKnown
			if o.Flags&flagGapKnown != 0 {
				r.GapMS = o.GapMS
				r.Flags |= flagGapKnown
			} else if r.GapMS == 0 && o.Position != 1 {
				r.Flags &^= flagGapKnown
			}
			out.Laps[i][o.Lap] = r
		}
		for _, o := range d.raceObservations {
			apply(o)
		}
		if d.raceLastSet {
			apply(d.raceLast)
		}
		for n, l := range d.Laps {
			if d.Classified && n > d.Completed {
				continue
			}
			r := out.Laps[i][n]
			r.Lap = n
			r.Time = l.Time
			r.Sectors = [3]uint32{l.S1, l.S2, l.S3}
			r.Compound = l.Compound
			if l.Time > 0 && !l.Valid {
				r.Flags |= flagInvalid
			}
			if d.Classified && n == d.Completed {
				r.Position = d.Position
			}
			out.Laps[i][n] = r
			if l.Time > 0 {
				field[n] = append(field[n], float64(l.Time)/1000)
				if n > out.MaxLap {
					out.MaxLap = n
				}
			}
		}
		if d.Classified && d.Completed > 0 {
			r := out.Laps[i][d.Completed]
			r.Lap, r.Position = d.Completed, d.Position
			out.Laps[i][d.Completed] = r
			out.MaxLap = max(out.MaxLap, d.Completed)
		}
	}
	for n, values := range field {
		sort.Float64s(values)
		mid := len(values) / 2
		v := values[mid]
		if len(values)%2 == 0 {
			v = (values[mid-1] + values[mid]) / 2
		}
		out.Median[n] = v
	}
	// At the finish the live UDP gap can become unavailable. For cars that
	// complete the winner's distance, final classified elapsed times (including
	// penalties) provide an authoritative finish gap. Never compare unlike laps.
	leader := -1
	for i, d := range m.Drivers {
		if d.Classified && d.Position == 1 && d.ResultStatus == 3 && d.Completed > 0 && d.RaceSeconds > 0 {
			leader = i
			break
		}
	}
	if leader >= 0 {
		winner := m.Drivers[leader]
		base := winner.RaceSeconds + float64(winner.PenaltySeconds)
		for i, d := range m.Drivers {
			if !d.Classified || d.ResultStatus != 3 || d.Completed != winner.Completed || d.RaceSeconds <= 0 {
				continue
			}
			r := out.Laps[i][d.Completed]
			gap := d.RaceSeconds + float64(d.PenaltySeconds) - base
			if gap >= 0 && r.Flags&flagGapKnown == 0 {
				r.GapMS = uint32(math.Round(gap * 1000))
				r.Flags |= flagGapKnown
				out.Laps[i][d.Completed] = r
			}
		}
	}
	m.raceMetrics = out
	return out
}
func raceFlags(flags uint8) string {
	labels := []string{}
	for _, v := range []struct {
		bit   uint8
		label string
	}{{flagPitIn, "IN"}, {flagPitOut, "OUT"}, {flagSC, "SC"}, {flagVSC, "VSC"}, {flagInvalid, "INV"}} {
		if flags&v.bit != 0 {
			labels = append(labels, v.label)
		}
	}
	return strings.Join(labels, " · ")
}

type RaceStint struct {
	Start, End, Compound int
	Open                 bool
}

func (m *AnalysisModel) raceStints(i int) []RaceStint {
	d := &m.Drivers[i]
	start := 1
	out := []RaceStint{}
	for _, st := range d.Stints {
		end := st.EndLap
		open := false
		if end == 255 {
			if d.Classified {
				end = d.Completed
			} else {
				end = 0
				for n, l := range d.Laps {
					if l.Time > 0 && n > end {
						end = n
					}
				}
				open = true
			}
		}
		if end >= start {
			out = append(out, RaceStint{start, end, st.Compound, open})
			start = end + 1
		}
	}
	return out
}
func averagePitLane(d *RecordedDriver) (float64, int) {
	var sum float64
	count := 0
	for _, p := range d.Pits {
		if p.Complete && !p.DriveThrough && p.StopMS > 0 && p.LaneMS > 0 {
			sum += float64(p.LaneMS) / 1000
			count++
		}
	}
	if count == 0 {
		return 0, 0
	}
	return sum / float64(count), count
}
func (m *AnalysisModel) markDriveThrough(driver int, clock float64) {
	if driver < 0 || driver >= len(m.Drivers) {
		return
	}
	d := &m.Drivers[driver]
	for i := len(d.Pits) - 1; i >= 0; i-- {
		p := &d.Pits[i]
		if p.LaneMS > 0 && clock >= p.Time && clock <= p.Time+float64(p.LaneMS)/1000+2 {
			p.DriveThrough = true
			return
		}
	}
}
func raceDriverIndices(m *AnalysisModel) []int {
	out := []int{}
	for i, d := range m.Drivers {
		if d.Name != "" || d.TeamKnown || d.Classified {
			out = append(out, i)
		}
	}
	return out
}
func driverCode(m *AnalysisModel, i int) string {
	name := strings.Fields(m.Drivers[i].name(i))
	s := ""
	if len(name) > 1 {
		for _, word := range name {
			r := []rune(word)
			if len(r) > 0 {
				s += string(r[0])
			}
		}
	} else if len(name) > 0 {
		s = name[0]
	}
	r := []rune(strings.ToUpper(s))
	if len(r) > 3 {
		r = r[:3]
	}
	if m.Drivers[i].RaceNumber > 0 {
		return fmt.Sprintf("%s #%d", string(r), m.Drivers[i].RaceNumber)
	}
	return string(r)
}
