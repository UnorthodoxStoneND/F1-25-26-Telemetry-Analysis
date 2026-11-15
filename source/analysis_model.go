package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"sort"
)

type TraceSample struct {
	Time, Distance, Speed, Throttle, Brake, RPM, Gear float64
	Lap, Frame                                        int
	LapMS                                             uint32
	Status                                            int
}
type RecordedLap struct {
	Number             int
	Time, S1, S2, S3   uint32
	Valid              bool
	Position, Compound int
	Wear               *[4]float64
	History            bool
}
type PitVisit struct {
	Lap                     int
	Time                    float64
	LaneMS, StopMS          uint32
	CompoundIn, CompoundOut int
	Complete                bool
	DriveThrough            bool
}
type RecordedStint struct{ EndLap, Compound int }
type pitPoint struct {
	Time          float64
	Lap, Compound int
	Active        bool
	Lane, Stop    uint32
}
type safetyPoint struct {
	Time      float64
	Lap, Kind int
}
type RecordedDriver struct {
	Name                                string
	Samples                             []TraceSample
	Laps                                map[int]*RecordedLap
	Pits                                []PitVisit
	Stints                              []RecordedStint
	Compound, Position, Grid, Completed int
	RaceSeconds                         float64
	Best                                uint32
	Wear                                *[4]float64
	lap                                 lapPoint
	pitActive                           bool
	pitPoints                           []pitPoint
	Restricted                          bool
	Classified                          bool
	TeamID                              int
	DriverID, RaceNumber                int
	MyTeam, TeamKnown                   bool
	ResultStatus, PenaltySeconds        int
	raceObservations                    []RaceObservation
	raceLast                            RaceObservation
	raceLastSet                         bool
	traceLaps                           []traceLapMark
	traceCompactFrame                   int
}
type lapPoint struct {
	Lap, Position int
	Distance      float64
	Current       uint32
	Valid         bool
	Clock         float64
	Status        int
}
type RecordedEvent struct {
	Code      string
	Time      float64
	Frame     uint32
	A, B, Lap int
	LapMS     uint32
}
type SessionConditions struct {
	Time                                    float64
	Track, Weather, Air, Surface, TotalLaps int
}
type AnalysisModel struct {
	Meta             RecordingMeta
	Drivers          [24]RecordedDriver
	Events           []RecordedEvent
	SCLaps, VSCLaps  map[int]bool
	RaceStart        *float64
	Truncated        bool
	Frames           int
	safety           int
	lapFrames        map[uint32][]lapPoint
	pending          map[uint32][]byte
	safetyPoints     []safetyPoint
	historyFloor     uint32
	Conditions       *SessionConditions
	conditionHistory []SessionConditions
	raceGate         RaceCaptureGate
	CompoundVisual   [256]int
	raceMetrics      *RaceMetrics
	traceScope       *analysisTraceScope
}

func loadAnalysis(dir string, meta RecordingMeta) (*AnalysisModel, error) {
	m := newModel(meta)
	partial, e := readJournalContext(context.Background(), filepath.Join(dir, meta.ID+".udp.gz"), true, m.ingest)
	m.Truncated = partial
	return m, e
}
func newModel(meta RecordingMeta) *AnalysisModel {
	m := &AnalysisModel{Meta: meta, SCLaps: map[int]bool{}, VSCLaps: map[int]bool{}, lapFrames: map[uint32][]lapPoint{}, pending: map[uint32][]byte{}}
	for i := range m.Drivers {
		m.Drivers[i].Laps = map[int]*RecordedLap{}
	}
	return m
}
func (d *RecordedDriver) lapRecord(n int) *RecordedLap {
	if d.Laps[n] == nil {
		d.Laps[n] = &RecordedLap{Number: n}
	}
	return d.Laps[n]
}
func (m *AnalysisModel) ingest(b []byte) {
	if len(b) < 29 {
		return
	}
	p := b[29:]
	id := b[6]
	slots := 22
	if m.Meta.Format == 2026 {
		slots = 24
	}
	clock := f32(b[15:])
	if !finite(clock) {
		return
	}
	frame := u32(b[23:])
	// A new journal persists the verified capture boundary, including fallback
	// starts whose earlier formation context was deliberately not written.
	if raceSession(m.Meta.Session) && !m.raceGate.Open && m.Meta.RaceCaptureFrame != nil && frame == *m.Meta.RaceCaptureFrame && m.Meta.RaceCaptureTrigger != "" {
		m.raceGate.Open = true
		m.raceGate.Frame = frame
		m.raceGate.Trigger = m.Meta.RaceCaptureTrigger
		m.raceGate.Lights = m.Meta.RaceStartLights
	}
	if !m.raceGate.allow(b, m.Meta.Session) {
		return
	}
	if int(b[27]) < slots {
		m.Meta.Player = int(b[27])
	}
	if (id == 11 || id == 15) && frame < m.historyFloor {
		return
	}
	m.Frames++
	m.raceMetrics = nil
	switch id {
	case 1:
		if len(p) >= 8 && p[6] > 0 && int8(p[7]) >= 0 {
			v := SessionConditions{clock, int(int8(p[7])), int(p[0]), int(int8(p[2])), int(int8(p[1])), int(p[3])}
			m.conditionHistory = append(m.conditionHistory, v)
			if m.Conditions == nil || clock >= m.Conditions.Time {
				m.Conditions = &v
			}
		}
		if len(p) > 124 {
			m.safety = int(p[124])
		}
	case 2:
		if len(p) < slots*57 {
			return
		}
		points := m.pairingPoints(frame, slots)
		for i := 0; i < slots; i++ {
			r := p[i*57:]
			d := &m.Drivers[i]
			n := int(r[33])
			dist := f32(r[20:])
			if !finite(dist) {
				continue
			}
			lp := lapPoint{Lap: n, Position: int(r[32]), Distance: dist, Current: u32(r[4:]), Valid: r[37] == 0, Clock: clock, Status: int(r[44])}
			points[i] = lp
			flags := uint8(0)
			if r[34] == 1 || (r[46] != 0 && !d.pitActive) {
				flags |= flagPitIn
			}
			if d.pitActive && r[46] == 0 {
				flags |= flagPitOut
			}
			if m.safety == 1 {
				flags |= flagSC
			}
			if m.safety == 2 {
				flags |= flagVSC
			}
			gap := uint32(u16(r[17:])) + uint32(r[19])*60000
			if r[32] > 0 && (gap > 0 || r[32] == 1) {
				flags |= flagGapKnown
			}
			if raceSession(m.Meta.Session) && n > 0 && r[32] > 0 {
				d.observeRace(RaceObservation{clock, frame, n, int(r[32]), gap, flags})
			}
			if n > 1 && u32(r) > 0 {
				l := d.lapRecord(n - 1)
				if !l.History || l.Time == 0 {
					l.Time = u32(r)
				}
			}
			if n > 0 {
				l := d.lapRecord(n)
				if !l.History {
					l.Valid = lp.Valid
				}
				if l.Position == 0 {
					l.Position = int(r[32])
				}
				l.Compound = d.Compound
				l.Wear = copyWear(d.Wear)
				if !l.History && r[36] >= 1 {
					l.S1 = uint32(u16(r[8:])) + uint32(r[10])*60000
				}
				if !l.History && r[36] >= 2 {
					l.S2 = uint32(u16(r[11:])) + uint32(r[13])*60000
				}
			}
			active := r[46] != 0
			if active || d.pitActive {
				point := pitPoint{clock, n, d.Compound, active, uint32(u16(r[47:])), uint32(u16(r[49:]))}
				d.pitPoints = append(d.pitPoints, point)
				d.applyPit(point)
			}
			d.lap = lp
			if !d.Classified {
				d.Position = int(r[32])
				d.Grid = int(r[43])
			}
		}
		m.lapFrames[frame] = points
		if packet := m.pending[frame]; packet != nil {
			m.telemetry(packet, points)
			delete(m.pending, frame)
		}
		// Frame pairing is exact. Packet loss produces a gap rather than assigning
		// telemetry to the wrong lap at the timing line.
		for f := range m.lapFrames {
			if f < frame && frame-f > 64 {
				delete(m.lapFrames, f)
			}
		}
		for f := range m.pending {
			if f < frame && frame-f > 64 {
				delete(m.pending, f)
			}
		}
		player := m.Meta.Player
		for i, p := range points {
			if p.Position == 1 {
				player = i
				break
			}
		}
		if player >= 0 && player < len(points) {
			lap := points[player].Lap
			if lap > 0 {
				if m.safety == 1 || m.safety == 2 {
					m.safetyPoints = append(m.safetyPoints, safetyPoint{clock, lap, m.safety})
				}
				if m.safety == 1 {
					m.SCLaps[lap] = true
				}
				if m.safety == 2 {
					m.VSCLaps[lap] = true
				}
			}
		}
	case 3:
		if len(p) < 4 {
			return
		}
		code := string(p[:4])
		if code == "FLBK" && len(p) >= 12 {
			m.trimZeroClockFuture(f32(p[8:]), u32(p[4:]))
			m.rewind(f32(p[8:]))
			m.historyFloor = frame
			return
		}
		e := RecordedEvent{Code: code, Time: clock, Frame: frame}
		if len(p) > 4 {
			e.A = int(p[4])
		}
		if len(p) > 5 {
			e.B = int(p[5])
		}
		if code == "FTLP" && len(p) >= 9 {
			v := f32(p[5:])
			if finite(v) && v > 0 && v < 3600 {
				e.LapMS = uint32(math.Round(v * 1000))
			}
		}
		if code == "SCAR" && len(p) >= 6 {
			if e.B <= 1 {
				m.safety = e.A
			} else {
				m.safety = 0
			}
		}
		if m.Meta.Player >= 0 && m.Meta.Player < 24 {
			e.Lap = m.Drivers[m.Meta.Player].lap.Lap
		}
		if code == "LGOT" {
			v := clock
			m.RaceStart = &v
		}
		if code == "DTSV" {
			m.markDriveThrough(e.A, clock)
		}
		if code == "OVTK" || code == "SCAR" || code == "STLG" || code == "LGOT" || code == "SEND" || code == "FTLP" || code == "DTSV" {
			for _, old := range m.Events {
				if old.Frame == e.Frame && old.Code == e.Code && old.A == e.A && old.B == e.B {
					return
				}
			}
			m.Events = append(m.Events, e)
		}
	case 4:
		stride, name := 57, 7
		if slots == 24 {
			stride, name = 60, 10
		}
		if len(p) < 1+slots*stride {
			return
		}
		for i := 0; i < slots; i++ {
			r := p[1+i*stride:]
			m.Drivers[i].Name = cleanName(r[name : name+32])
			m.Drivers[i].TeamID, m.Drivers[i].MyTeam = int(r[3]), r[4] == 1
			m.Drivers[i].DriverID, m.Drivers[i].RaceNumber = int(r[1]), int(r[5])
			if slots == 24 {
				m.Drivers[i].TeamID, m.Drivers[i].MyTeam = u16(r[5:]), r[7] == 1
				m.Drivers[i].DriverID, m.Drivers[i].RaceNumber = u16(r[1:]), int(r[8])
			}
			m.Drivers[i].TeamKnown = i < int(p[0])
			privacy := 39
			if slots == 24 {
				privacy = 42
			}
			m.Drivers[i].Restricted = r[0] == 0 && r[privacy] == 0 && i != m.Meta.Player
		}
	case 6:
		if points := m.lapFrames[frame]; points != nil {
			m.telemetry(b, points)
		} else {
			m.pending[frame] = append([]byte(nil), b...)
			for f := range m.pending {
				if f < frame && frame-f > 64 {
					delete(m.pending, f)
				}
			}
		}
	case 7:
		stride := 55
		if slots == 24 {
			stride = 59
		}
		if len(p) < slots*stride {
			return
		}
		for i := 0; i < slots; i++ {
			m.Drivers[i].Compound = int(p[i*stride+25])
			m.CompoundVisual[p[i*stride+25]] = int(p[i*stride+26])
		}
	case 8:
		if len(p) < 1 {
			return
		}
		count := int(p[0])
		if count > slots || len(p) < 1+count*46 {
			return
		}
		for i := 0; i < count; i++ {
			r := p[1+i*46:]
			d := &m.Drivers[i]
			d.Classified = true
			d.Position, d.Completed, d.Grid = int(r[0]), int(r[1]), int(r[2])
			d.ResultStatus, d.PenaltySeconds = int(r[5]), int(r[19])
			d.Best = u32(r[7:])
			v := math.Float64frombits(binary.LittleEndian.Uint64(r[11:]))
			if finite(v) && v >= 0 {
				d.RaceSeconds = v
			}
			d.Stints = nil
			for j := 0; j < int(r[21]) && j < 8; j++ {
				d.Stints = append(d.Stints, RecordedStint{int(r[38+j]), int(r[22+j])})
				m.CompoundVisual[r[22+j]] = int(r[30+j])
			}
		}
	case 10:
		if len(p) < slots*46 {
			return
		}
		for i := 0; i < slots; i++ {
			if m.Drivers[i].Restricted {
				continue
			}
			var w [4]float64
			valid := true
			for j := range w {
				w[j] = f32(p[i*46+j*4:])
				valid = valid && finite(w[j]) && w[j] >= 0 && w[j] <= 100
			}
			if valid {
				m.Drivers[i].Wear = &w
			}
		}
	case 11:
		if len(p) < 1431 || int(p[0]) >= slots {
			return
		}
		d := &m.Drivers[int(p[0])]
		count := int(p[1])
		if count > 100 {
			count = 100
		}
		if !d.Classified {
			d.Best = 0
		}
		for j := 0; j < count; j++ {
			r := p[7+j*14:]
			l := d.lapRecord(j + 1)
			l.History = true
			l.Time = u32(r)
			l.S1 = uint32(u16(r[4:])) + uint32(r[6])*60000
			l.S2 = uint32(u16(r[7:])) + uint32(r[9])*60000
			l.S3 = uint32(u16(r[10:])) + uint32(r[12])*60000
			l.Valid = r[13]&1 != 0
			if !d.Classified && l.Valid && l.Time > 0 && (d.Best == 0 || l.Time < d.Best) {
				d.Best = l.Time
			}
		}
		if !d.Classified {
			d.Stints = nil
			for j := 0; j < int(p[2]) && j < 8; j++ {
				r := p[1407+j*3:]
				d.Stints = append(d.Stints, RecordedStint{int(r[0]), int(r[1])})
			}
		}
		for j := 0; j < int(p[2]) && j < 8; j++ {
			r := p[1407+j*3:]
			m.CompoundVisual[r[1]] = int(r[2])
		}
	case 15:
		if len(p) < 2 {
			return
		}
		count := int(p[0])
		if count > 50 || len(p) < 2+count*slots {
			return
		}
		for j := 0; j < count; j++ {
			for i := 0; i < slots; i++ {
				m.Drivers[i].lapRecord(int(p[1]) + j + 1).Position = int(p[2+j*slots+i])
			}
		}
	}
}
func cleanName(b []byte) string {
	for i, v := range b {
		if v == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
func copyWear(w *[4]float64) *[4]float64 {
	if w == nil {
		return nil
	}
	v := *w
	return &v
}
func (m *AnalysisModel) telemetry(b []byte, points []lapPoint) {
	p := b[29:]
	stride := 60
	if m.Meta.Format == 2026 {
		stride = 59
	}
	if len(p) < len(points)*stride {
		return
	}
	for i, lp := range points {
		if m.Drivers[i].Restricted {
			continue
		}
		gridStart := m.Meta.Session >= 15 && m.Meta.Session <= 17 && lp.Lap == 1 && lp.Distance >= -2000
		if lp.Lap < 1 || (lp.Distance < 0 && !gridStart) {
			continue
		}
		r := p[i*stride:]
		s := TraceSample{Time: f32(b[15:]), Distance: lp.Distance, Speed: float64(u16(r)), Throttle: f32(r[2:]) * 100, Brake: f32(r[10:]) * 100, RPM: float64(u16(r[16:])), Gear: float64(int8(r[15])), Lap: lp.Lap, Frame: int(u32(b[23:])), LapMS: lp.Current, Status: lp.Status}
		if !finite(s.Throttle) || !finite(s.Brake) || s.Throttle < 0 || s.Throttle > 100.1 || s.Brake < 0 || s.Brake > 100.1 {
			continue
		}
		d := &m.Drivers[i]
		d.noteTraceLap(s)
		if m.traceScope != nil && !m.traceScope.captures(i, lp.Lap) {
			continue
		}
		k := sort.Search(len(d.Samples), func(j int) bool {
			return d.Samples[j].Time > s.Time || (d.Samples[j].Time == s.Time && d.Samples[j].Frame >= s.Frame)
		})
		if k < len(d.Samples) && d.Samples[k].Frame == s.Frame {
			continue
		}
		d.Samples = append(d.Samples, TraceSample{})
		copy(d.Samples[k+1:], d.Samples[k:])
		d.Samples[k] = s
	}
}

// Waiting at the start lights can span many frames at session time zero.
// A flashback to that time must also remove later frames, not only later times.
func (m *AnalysisModel) trimZeroClockFuture(clock float64, frame uint32) {
	for i := range m.Drivers {
		m.Drivers[i].rewindRaceObservations(clock, &frame)
		m.Drivers[i].trimTraceLaps(clock, &frame)
		kept := m.Drivers[i].Samples[:0]
		for _, s := range m.Drivers[i].Samples {
			if s.Time != clock || s.Frame <= int(frame) {
				kept = append(kept, s)
			}
		}
		m.Drivers[i].Samples = kept
	}
	kept := m.Events[:0]
	for _, e := range m.Events {
		if e.Time != clock || e.Frame <= frame {
			kept = append(kept, e)
		}
	}
	m.Events = kept
}

func (m *AnalysisModel) stintEndLabel(d *RecordedDriver, st RecordedStint, start int) string {
	if st.EndLap != 255 {
		return knownInt(st.EndLap)
	}
	if d.Classified {
		if d.Completed >= start {
			return fmt.Sprint(d.Completed)
		}
		return "Ended / no completed lap"
	}
	switch m.Meta.State {
	case "completed":
		return "Session ended / final lap unknown"
	case "interrupted", "restarted":
		return "Capture ended / final lap unknown"
	}
	return "In progress"
}
func (m *AnalysisModel) rewind(clock float64) {
	if !finite(clock) {
		return
	}
	m.raceMetrics = nil
	conditions := m.conditionHistory[:0]
	m.Conditions = nil
	for _, v := range m.conditionHistory {
		if v.Time <= clock {
			conditions = append(conditions, v)
			if m.Conditions == nil || v.Time >= m.Conditions.Time {
				copy := v
				m.Conditions = &copy
			}
		}
	}
	m.conditionHistory = conditions
	m.lapFrames = map[uint32][]lapPoint{}
	m.pending = map[uint32][]byte{}
	for i := range m.Drivers {
		d := &m.Drivers[i]
		d.rewindRaceObservations(clock, nil)
		k := sort.Search(len(d.Samples), func(j int) bool { return d.Samples[j].Time > clock })
		d.Samples = d.Samples[:k]
		lap := 0
		if k > 0 {
			lap = d.Samples[k-1].Lap
		}
		if m.traceScope != nil {
			lap = d.traceLapAt(clock)
		}
		d.trimTraceLaps(clock, nil)
		for n := range d.Laps {
			if n >= lap {
				delete(d.Laps, n)
			}
		}
		k = 0
		for _, p := range d.pitPoints {
			if p.Time <= clock {
				d.pitPoints[k] = p
				k++
			}
		}
		d.pitPoints = d.pitPoints[:k]
		d.Pits = nil
		d.pitActive = false
		for _, p := range d.pitPoints {
			d.applyPit(p)
		}
		d.lap = lapPoint{}
		d.RaceSeconds = 0
		d.Best = 0
		d.Completed = 0
		d.Classified = false
		d.ResultStatus, d.PenaltySeconds = 0, 0
		d.Stints = nil
	}
	ev := m.Events[:0]
	for _, e := range m.Events {
		if e.Time <= clock {
			ev = append(ev, e)
		}
	}
	m.Events = ev
	for _, e := range m.Events {
		if e.Code == "DTSV" {
			m.markDriveThrough(e.A, e.Time)
		}
	}
	m.SCLaps = map[int]bool{}
	m.VSCLaps = map[int]bool{}
	points := m.safetyPoints[:0]
	for _, p := range m.safetyPoints {
		if p.Time <= clock {
			points = append(points, p)
			if p.Kind == 1 {
				m.SCLaps[p.Lap] = true
			} else {
				m.VSCLaps[p.Lap] = true
			}
		}
	}
	m.safetyPoints = points
	m.safety = 0
	if m.RaceStart != nil && *m.RaceStart > clock {
		m.RaceStart = nil
	}
}
func (d *RecordedDriver) applyPit(p pitPoint) {
	if p.Active && !d.pitActive {
		d.Pits = append(d.Pits, PitVisit{Lap: p.Lap, Time: p.Time, CompoundIn: p.Compound})
	}
	if len(d.Pits) > 0 && (p.Active || d.pitActive) {
		v := &d.Pits[len(d.Pits)-1]
		if p.Lane > v.LaneMS {
			v.LaneMS = p.Lane
		}
		if p.Stop > v.StopMS {
			v.StopMS = p.Stop
		}
		v.CompoundOut = p.Compound
		v.Complete = !p.Active
	}
	d.pitActive = p.Active
}
func (d *RecordedDriver) name(i int) string {
	if d.Name != "" {
		return d.Name
	}
	return fmt.Sprintf("Driver %d", i+1)
}
func (d *RecordedDriver) laps() []int {
	v := []int{}
	for n := range d.Laps {
		v = append(v, n)
	}
	sort.Ints(v)
	return v
}
func (d *RecordedDriver) trace(lap int) []TraceSample {
	return d.traceRun(lap, false)
}
func (d *RecordedDriver) traceRun(lap int, includePit bool) []TraceSample {
	// Practice/qualifying can keep a lap number through an aborted flying lap,
	// garage stay, out-lap and a fresh attempt. Lap number alone is not a run ID.
	// Keep the latest timed attempt, never overlay those different traversals.
	var selected, run []TraceSample
	finish := func() {
		if len(run) > 0 {
			selected = run
			run = nil
		}
	}
	for _, s := range d.Samples {
		if s.Lap != lap {
			continue
		}
		if s.Status != 1 && s.Status != 4 && !(includePit && (s.Status == 2 || s.Status == 3)) {
			finish()
			continue
		}
		if len(run) > 0 {
			previous := run[len(run)-1]
			if s.Distance < previous.Distance-100 || uint64(s.LapMS)+500 < uint64(previous.LapMS) {
				finish()
			}
		}
		run = append(run, s)
	}
	finish()
	return selected
}

func traceCoverage(samples []TraceSample, lapMS uint32) string {
	if len(samples) == 0 {
		return "No timed-run samples"
	}
	first, last := samples[0], samples[len(samples)-1]
	if first.LapMS > 1000 || first.Distance > 100 {
		return "Partial trace: recording starts after the timing line"
	}
	if lapMS > 0 && uint64(last.LapMS)+1000 < uint64(lapMS) {
		return "Partial trace: lap completion was not transmitted"
	}
	if lapMS == 0 {
		return "Unfinished lap"
	}
	return ""
}
