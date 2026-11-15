package main

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"sort"
	"strings"
)

func practiceSession(kind int) bool { return kind >= 1 && kind <= 4 }
func practiceValues(d *RecordedDriver) []float64 {
	v := []float64{}
	for _, l := range d.Laps {
		if l.Valid && l.Time > 0 {
			v = append(v, float64(l.Time)/1000)
		}
	}
	sort.Float64s(v)
	return v
}
func practiceTimes(d *RecordedDriver) (uint32, uint32, int) {
	best := d.Best
	var total uint64
	valid, count := 0, 0
	for _, l := range d.Laps {
		if l.Time == 0 {
			continue
		}
		count++
		if !l.Valid {
			continue
		}
		if best == 0 || l.Time < best {
			best = l.Time
		}
		total += uint64(l.Time)
		valid++
	}
	count = max(count, d.Completed)
	if valid == 0 {
		return best, 0, count
	}
	return best, uint32(math.Round(float64(total) / float64(valid))), count
}
func practiceCompounds(d *RecordedDriver) string {
	ids := map[int]bool{}
	for _, l := range d.Laps {
		if l.Time > 0 && compound(l.Compound) != "—" {
			ids[l.Compound] = true
		}
	}
	for _, s := range d.Stints {
		if compound(s.Compound) != "—" {
			ids[s.Compound] = true
		}
	}
	ordered := []int{}
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Ints(ordered)
	labels := []string{}
	for _, id := range ordered {
		labels = append(labels, compound(id))
	}
	if len(labels) == 0 {
		return "—"
	}
	return strings.Join(labels, " · ")
}
func (a *App) practiceResults(m *AnalysisModel, ordered []int) M {
	leader := uint32(0)
	for _, i := range ordered {
		best, _, _ := practiceTimes(&m.Drivers[i])
		if best > 0 && (leader == 0 || best < leader) {
			leader = best
		}
	}
	rows := [][]any{}
	for _, i := range ordered {
		d := m.Drivers[i]
		best, avg, laps := practiceTimes(&d)
		gap := "—"
		if best > 0 && leader > 0 && best > leader {
			gap = fmt.Sprintf("+%.3f s", float64(best-leader)/1000)
		}
		rows = append(rows, []any{a.podiumPosition(d.Position), resultDriverName(d, i), m.teamName(d), practiceCompounds(&d), fmt.Sprint(laps), lapTime(best), gap, lapTime(avg)})
	}
	table := tableBlock("analysis-results", []string{"POS", "Full Driver Name", "Team", "Compounds", "Laps", "Fastest Lap", "Gap", "Avg Time"}, rows)
	columns := table["table"].(M)["columns"].([]M)
	for i, w := range []int{60, 220, 350, 140, 65, 125, 120, 125} {
		columns[i]["width"] = w
	}
	return table
}
func (a *App) analysisDriverChips(m *AnalysisModel) []M {
	chips := []M{{"id": "analysis-race-all", "label": "All", "style": "ghost"}, {"id": "analysis-race-none", "label": "Clear", "style": "ghost"}}
	for _, i := range raceDriverIndices(m) {
		style := "outline"
		if a.analysis.RaceDrivers&(1<<uint(i)) != 0 {
			style = "primary"
		}
		chips = append(chips, M{"id": fmt.Sprintf("analysis-race-driver-%s-%d", m.Meta.ID, i), "label": m.Drivers[i].name(i), "style": style})
	}
	return []M{{"buttons": chips}}
}
func (a *App) practiceAnalysisBlocks(m *AnalysisModel) []M {
	a.ensureRaceSelection(m)
	out := []M{a.raceImage(m, "practice-team-sectors", func(path string) error { return renderTeamSectorLeaders(path, m) })}
	out = append(out, a.analysisDriverChips(m)...)
	selected := a.selectedRaceDrivers(m)
	if len(selected) == 0 {
		return append(out, textBlock("No drivers selected. Choose drivers for lap-time distribution and consistency analysis."))
	}
	out = append(out, a.raceImage(m, "legend", func(path string) error { return renderRaceLegend(path, m, selected) }))
	out = append(out, a.raceImage(m, "practice-distribution", func(path string) error {
		filtered := newModel(m.Meta)
		for _, i := range selected {
			filtered.Drivers[i] = m.Drivers[i]
			filtered.Drivers[i].Laps = map[int]*RecordedLap{}
			for n, l := range m.Drivers[i].Laps {
				if l.Valid && l.Time > 0 {
					filtered.Drivers[i].Laps[n] = l
				}
			}
		}
		return renderPracticeDistribution(path, filtered, selected)
	}), a.raceImage(m, "practice-consistency", func(path string) error { return renderPracticeConsistency(path, m, selected) }))
	return out
}

type practiceTeamSector struct {
	Name   string
	Driver int
	Time   uint32
}

func teamSectorLeaders(m *AnalysisModel) [3][]practiceTeamSector {
	var groups [3]map[string]practiceTeamSector
	colours := map[string]int{}
	for j := range groups {
		groups[j] = map[string]practiceTeamSector{}
	}
	for i, d := range m.Drivers {
		if d.Name == "" {
			continue
		}
		name := m.teamName(d)
		key := fmt.Sprintf("%t:%d", d.MyTeam, d.TeamID)
		if !d.TeamKnown {
			key = fmt.Sprint("unknown:", i)
		}
		if _, ok := colours[key]; !ok {
			colours[key] = i
		}
		if name == "—" {
			if d.TeamKnown {
				name = fmt.Sprintf("Team %d", d.TeamID)
			} else {
				name = "Unknown team · " + driverCode(m, i)
			}
		}
		for _, l := range d.Laps {
			if !l.Valid || l.Time == 0 {
				continue
			}
			for j, v := range [3]uint32{l.S1, l.S2, l.S3} {
				if v == 0 {
					continue
				}
				previous, ok := groups[j][key]
				if !ok || v < previous.Time {
					groups[j][key] = practiceTeamSector{name, colours[key], v}
				}
			}
		}
	}
	var out [3][]practiceTeamSector
	for j, g := range groups {
		for _, v := range g {
			out[j] = append(out[j], v)
		}
		sort.Slice(out[j], func(a, b int) bool {
			if out[j][a].Time == out[j][b].Time {
				return out[j][a].Name < out[j][b].Name
			}
			return out[j][a].Time < out[j][b].Time
		})
	}
	return out
}
func renderTeamSectorLeaders(path string, m *AnalysisModel) error {
	leaders := teamSectorLeaders(m)
	f := newRaceFigure(3600, 620, "Team sector leaders", "Top five teams per sector · best recorded sector from valid completed laps · all drivers")
	for sector, teams := range leaders {
		left := 40 + sector*1190
		f.text(left, 115, 1100, 38, fmt.Sprintf("SECTOR %d", sector+1), 24, 600, raceMuted)
		if len(teams) == 0 {
			f.text(left, 190, 1100, 42, "No valid sector times recorded", 25, 400, raceMuted)
			continue
		}
		for row, v := range teams[:min(5, len(teams))] {
			y := 175 + row*78
			c := raceColours[v.Driver%len(raceColours)]
			f.text(left+10, y, 835, 40, v.Name, 25, 600, c)
			label := fmt.Sprintf("+%.3f", float64(v.Time-teams[0].Time)/1000)
			if row == 0 {
				label = fmt.Sprintf("%.3f s", float64(v.Time)/1000)
			}
			f.text(left+865, y, 270, 40, label, 26, 600, raceFG)
			chartLine(f.im, left+10, y+51, left+1130, y+51, raceGrid)
			length := int(1120 * float64(teams[0].Time) / float64(v.Time))
			chartLine(f.im, left+10, y+51, left+10+length, y+51, c)
		}
	}
	return f.save(path)
}
func renderPracticeDistribution(path string, m *AnalysisModel, selected []int) error {
	return renderLapDistributionWithCaption(path, m, selected, 3600, "Valid completed laps only · box: middle 50% / median · whiskers: 1.5×IQR")
}
func practiceConsistency(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	sum := 0.
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	variance := 0.
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	return mean, math.Sqrt(variance / float64(len(values)))
}
func renderPracticeConsistency(path string, m *AnalysisModel, selected []int) error {
	f := newRaceFigure(3600, 900, "Consistency analysis", "Valid completed laps · box: mean ± standard deviation · whiskers: minimum / maximum · at least two laps required")
	f.category = true
	f.timeAxis = true
	f.plot.Max.Y -= 90
	low, high := math.Inf(1), math.Inf(-1)
	values := map[int][]float64{}
	for _, i := range selected {
		v := practiceValues(&m.Drivers[i])
		values[i] = v
		if len(v) >= 2 {
			low = math.Min(low, v[0])
			high = math.Max(high, v[len(v)-1])
		}
	}
	if !finite(low) {
		f.text(150, 230, 3200, 55, "At least two valid completed laps per driver are needed to measure consistency.", 29, 400, raceMuted)
		return f.save(path)
	}
	pad := math.Max(1, (high-low)*.1)
	f.axes(.5, float64(max(1, len(selected)))+.5, low-pad, high+pad, "Driver", "Lap time · M:SS", false)
	for n, i := range selected {
		x, _ := f.xy(float64(n+1), low)
		slot := f.plot.Dx() / max(1, len(selected))
		half := min(160, slot/3)
		c := raceColours[i%len(raceColours)]
		f.text(x-slot/2+10, f.plot.Max.Y+15, slot-20, 38, driverCode(m, i), 25, 600, c)
		v := values[i]
		if len(v) < 2 {
			f.text(x-slot/2+10, f.plot.Max.Y+55, slot-20, 35, "Insufficient laps", 21, 400, raceMuted)
			continue
		}
		mean, sd := practiceConsistency(v)
		_, yMin := f.xy(float64(n+1), v[0])
		_, yMax := f.xy(float64(n+1), v[len(v)-1])
		chartLine(f.im, x, yMin, x, yMax, c)
		chartLine(f.im, x-half/2, yMin, x+half/2, yMin, c)
		chartLine(f.im, x-half/2, yMax, x+half/2, yMax, c)
		_, y0 := f.xy(float64(n+1), math.Max(v[0], mean-sd))
		_, y1 := f.xy(float64(n+1), math.Min(v[len(v)-1], mean+sd))
		draw.Draw(f.im, image.Rect(x-half, y1, x+half, max(y0, y1+2)), &image.Uniform{c}, image.Point{}, draw.Src)
		_, mid := f.xy(float64(n+1), mean)
		chartLine(f.im, x-half, mid, x+half, mid, raceFG)
		f.text(x-slot/2+10, f.plot.Max.Y+55, slot-20, 35, fmt.Sprintf("SD %.3fs", sd), 21, 400, raceMuted)
		f.text(x-slot/2+10, f.plot.Max.Y+88, slot-20, 30, fmt.Sprintf("%d valid laps", len(v)), 18, 400, raceMuted)
	}
	return f.save(path)
}
