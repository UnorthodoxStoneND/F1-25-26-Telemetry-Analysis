package main

import (
	"fmt"
	"strings"
)

type sessionLayout uint8

const (
	layoutPractice sessionLayout = iota
	layoutQualifying
	layoutRace
)

func layoutFor(f Frame) sessionLayout {
	switch {
	case f.SessionType >= 5 && f.SessionType <= 14:
		return layoutQualifying
	case f.SessionType >= 15 && f.SessionType <= 17:
		return layoutRace
	case f.SessionType == 0:
		if f.SessionKind == 2 {
			return layoutQualifying
		}
		if f.SessionKind == 3 || f.SessionKind == 4 {
			return layoutRace
		}
	}
	// Time Trial and unknown sessions retain the basic timed-session layout.
	return layoutPractice
}

var practiceColumns = makeColumns(layoutPractice)
var qualifyingColumns = makeColumns(layoutQualifying)
var raceColumns = makeColumns(layoutRace)

func columns(layout sessionLayout) []M {
	switch layout {
	case layoutQualifying:
		return qualifyingColumns
	case layoutRace:
		return raceColumns
	default:
		return practiceColumns
	}
}
func makeColumns(layout sessionLayout) []M {
	c := []M{{"key": "position", "title": "Pos", "width": 40}}
	if layout == layoutRace {
		c = append(c, M{"key": "gain", "title": "±", "width": 40})
	}
	c = append(c, M{"key": "team", "title": "", "width": 38}, M{"key": "driver", "title": "Driver"}, M{"key": "tyre", "title": "Tyre", "width": 68})
	if layout == layoutRace {
		c = append(c, M{"key": "tyre_age", "title": "Tyre age", "width": 68}, M{"key": "gap", "title": "Gap ahead"}, M{"key": "last", "title": "Last lap"}, M{"key": "best", "title": "Best lap"})
		for _, key := range []string{"S1", "S2", "S3", "FL", "FR", "RL", "RR", "Avg"} {
			title := key
			if key != "S1" && key != "S2" && key != "S3" {
				title += "%"
			}
			c = append(c, M{"key": key, "title": title})
		}
	} else {
		c = append(c, M{"key": "laps", "title": "Laps", "width": 52})
		if layout == layoutQualifying {
			c = append(c, M{"key": "gap", "title": "Gap to best"})
		}
		c = append(c, M{"key": "last", "title": "Last lap"}, M{"key": "best", "title": "Best lap"})
		if layout == layoutQualifying {
			for _, key := range []string{"S1", "S2", "S3"} {
				c = append(c, M{"key": key, "title": key})
			}
		}
	}
	if layout != layoutPractice {
		c = append(c, M{"key": "pen", "title": "PEN", "width": 80})
	}
	c = append(c, M{"key": "status", "title": "Status", "width": 76})
	if layout != layoutRace {
		c = append(c, M{"key": "setup", "title": "Setup", "width": 64})
	}
	return c
}

func sessionBest(f Frame) uint32 {
	var best uint32
	for _, c := range f.Cars {
		if c.Known&knownBest != 0 && c.Best > 0 && retirementLabel(c) != "DSQ" && (best == 0 || c.Best < best) {
			best = c.Best
		}
	}
	return best
}
func statusCell(c Car) any {
	if label := retirementLabel(c); label != "" {
		return textValue{Text: label, Tone: "muted"}
	}
	if c.Known&knownPit != 0 && c.Flags&inPitLane != 0 {
		return textValue{Text: "Pit", Tone: "warn"}
	}
	if c.Known&knownResult != 0 {
		switch (c.Flags & resultMask) >> 8 {
		case 2:
			if c.Flags&outLap != 0 {
				return textValue{Text: "Out", Tone: "warn"}
			}
			return textValue{Text: "Run", Tone: "good"}
		case 3:
			return textValue{Text: "Finished", Tone: "neutral"}
		case 6:
			return textValue{Text: "NC", Tone: "muted"}
		}
	}
	return "—"
}
func gainCell(c Car) any {
	if c.Known&(knownGrid|knownPosition) != (knownGrid|knownPosition) || c.Grid <= 0 || c.Grid > maxCars || c.Position <= 0 || c.Position > maxCars {
		return "—"
	}
	gain := c.Grid - c.Position
	tone, text := "muted", "—"
	if gain > 0 {
		tone = "good"
		text = fmt.Sprintf("+%d", gain)
	} else if gain < 0 {
		tone = "bad"
		text = fmt.Sprint(gain)
	}
	return M{"text": text, "sort": gain, "tone": tone}
}
func gapCell(ms uint32, known bool, leader bool) any {
	if !known || leader {
		return "—"
	}
	return M{"text": fmt.Sprintf("+%d.%03d", ms/1000, ms%1000), "sort": ms}
}
func sectorCell(c Car, index int, layout sessionLayout) any {
	known, sectors := uint32(knownSectors), c.BestSectors
	if layout == layoutRace {
		known, sectors = knownLastSectors, c.LastSectors
	}
	if c.Known&known == 0 {
		return "—"
	}
	ms := sectors[index]
	if ms == 0 {
		return "—"
	}
	text := fmt.Sprintf("%d.%03d", ms/1000, ms%1000)
	if ms >= 60000 {
		text = lapTime(ms, true)
	}
	return M{"text": text, "sort": ms}
}
func driverCells(c Car, layout sessionLayout, best uint32, open bool) []any {
	p := "—"
	if position(c) < 1000 {
		p = fmt.Sprint(position(c))
	}
	cells := []any{p}
	if layout == layoutRace {
		cells = append(cells, gainCell(c))
	}
	cells = append(cells, badgeCell(c), driverName(c), tyreCell(c))
	if layout == layoutRace {
		var age any = "—"
		if c.Known&knownTyreAge != 0 {
			age = M{"text": fmt.Sprint(c.TyreAgeLaps), "sort": c.TyreAgeLaps}
		}
		cells = append(cells, age)
		known := c.Known&knownGapAhead != 0 && c.GapAhead >= 0 && retirementLabel(c) == ""
		cells = append(cells, gapCell(uint32(c.GapAhead), known, c.Known&knownPosition != 0 && c.Position == 1),
			lapTime(c.Last, c.Known&knownLast != 0), lapTime(c.Best, c.Known&knownBest != 0))
		for sector := 0; sector < 3; sector++ {
			cells = append(cells, sectorCell(c, sector, layout))
		}
		for _, wheel := range []int{2, 3, 0, 1} {
			cells = append(cells, wearCell(c.Wear[wheel], c.WearKnown&(1<<wheel) != 0))
		}
		avg := (c.Wear[0] + c.Wear[1] + c.Wear[2] + c.Wear[3]) / 4
		cells = append(cells, wearCell(avg, c.WearKnown&15 == 15))
	} else {
		var laps any = "—"
		if c.Known&knownLaps != 0 {
			laps = M{"text": fmt.Sprint(c.Laps), "sort": c.Laps}
		}
		cells = append(cells, laps)
		if layout == layoutQualifying {
			known := c.Known&knownBest != 0 && best > 0 && c.Best >= best && retirementLabel(c) != "DSQ"
			cells = append(cells, gapCell(c.Best-best, known, c.Best == best))
		}
		cells = append(cells, lapTime(c.Last, c.Known&knownLast != 0), lapTime(c.Best, c.Known&knownBest != 0))
		if layout == layoutQualifying {
			for sector := 0; sector < 3; sector++ {
				cells = append(cells, sectorCell(c, sector, layout))
			}
		}
	}
	if layout != layoutPractice {
		cells = append(cells, penaltyCell(c))
	}
	if layout == layoutRace {
		c.Flags &^= outLap
	}
	cells = append(cells, statusCell(c))
	if layout != layoutRace {
		cells = append(cells, toggleValue{Edit: toggleEdit{Toggle: toggleField{Value: open}}})
	}
	return cells
}

// DT/SG are pending counts. Seconds are the current classification total,
// explicitly requested by the user even though not an unserved-only balance.
func penaltyCell(c Car) any {
	parts := []string{}
	if c.Known&knownTimePenalty != 0 && c.TimePenaltyMS > 0 {
		time := fmt.Sprintf("%d.%03d", c.TimePenaltyMS/1000, c.TimePenaltyMS%1000)
		parts = append(parts, strings.TrimRight(strings.TrimRight(time, "0"), ".")+"s")
	}
	if c.Known&knownPenalties != 0 {
		for _, p := range []struct {
			count uint8
			label string
		}{{c.DriveThrough, "DT"}, {c.StopGo, "SG"}} {
			if p.count == 1 {
				parts = append(parts, p.label)
			} else if p.count > 1 {
				parts = append(parts, fmt.Sprintf("%d%s", p.count, p.label))
			}
		}
	}
	if len(parts) == 0 {
		return "—"
	}
	return textValue{Text: strings.Join(parts, "+"), Tone: "warn"}
}
