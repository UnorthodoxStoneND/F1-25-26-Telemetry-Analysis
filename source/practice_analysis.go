package main

import (
	"fmt"
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
	out := []M{nativeTeamSectors(m)}
	out = append(out, a.analysisDriverChips(m)...)
	selected := a.selectedRaceDrivers(m)
	if len(selected) == 0 {
		return append(out, textBlock("No drivers selected. Choose drivers for lap-time distribution and consistency analysis."))
	}
	return append(out, nativeDistribution(m, selected, true, false), nativeDistribution(m, selected, true, true))
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
