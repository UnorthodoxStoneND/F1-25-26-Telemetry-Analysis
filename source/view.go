package main

import (
	"cmp"
	"fmt"
	"slices"
)

func textBlock(s string) M { return M{"text": M{"text": s, "tone": "neutral"}} }
func lapTime(ms uint32, known bool) string {
	if !known || ms == 0 {
		return "—"
	}
	return fmt.Sprintf("%d:%02d.%03d", ms/60000, ms/1000%60, ms%1000)
}
func driverName(c Car) string {
	if c.Known&knownName != 0 && c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("Driver %d", c.Vehicle+1)
}
func rowID(f Frame, c Car) string {
	return fmt.Sprintf("car-%d-%d-%d", f.Session, c.Vehicle, c.Identity)
}
func retirementLabel(c Car) string {
	if c.Known&knownResult != 0 {
		switch (c.Flags & resultMask) >> 8 {
		case 4, 7:
			return "DNF"
		case 5:
			return "DSQ"
		}
	}
	return ""
}
func position(c Car) int {
	if c.Known&knownPosition != 0 && c.Position > 0 {
		return int(c.Position)
	}
	if c.Known&knownGrid != 0 && c.Grid > 0 {
		return int(c.Grid)
	}
	return 1000 + int(c.Vehicle)
}
func visible(f Frame, mode string) []Car {
	return visibleInto(f, mode, nil)
}
func visibleInto(f Frame, mode string, storage []Car) []Car {
	cars := append(storage, f.Cars...)
	slices.SortStableFunc(cars, func(a, b Car) int { return cmp.Compare(position(a), position(b)) })
	if mode == "full" || len(cars) <= 5 {
		return cars
	}
	player := -1
	for n, c := range cars {
		if int32(c.Vehicle) == f.Player {
			player = n
			break
		}
	}
	// Do not guess which car belongs to the user while controllers are loading.
	if player < 0 {
		return nil
	}
	start := player - 2
	if start < 0 {
		start = 0
	}
	if start+5 > len(cars) {
		start = len(cars) - 5
	}
	return cars[start : start+5]
}

// Only retain inputs to the displayed page, not the sample clock or hidden
// setups. Poll every tick as before, but leave Swofty's existing view alone
// until one of its inputs changes. Fixed storage bounds the retained state.
type viewState struct {
	Layout                             sessionLayout
	Hidden                             uint32
	Disabled, ColumnsOpen              bool
	SessionBest                        uint32
	Session                            uint64
	SessionLabel                       string
	Player                             int32
	Status, Diagnostic, Mode, Expanded string
	Highlight, Active                  bool
	Count                              int
	Cars                               [maxCars]Car
}

func (a *App) viewState() viewState {
	layout := layoutFor(a.frame)
	state := viewState{Layout: layout, Hidden: a.hidden[layout], Disabled: a.disabled[layout], ColumnsOpen: a.columnsOpen, Session: a.frame.Session, Player: a.frame.Player,
		SessionLabel: a.pageSubtitle(),
		Status:       a.status, Diagnostic: a.frame.Diagnostic, Mode: a.mode,
		Expanded: a.expanded, Highlight: a.highlight, Active: a.frame.State&active != 0}

	if a.disabled[layout] || len(a.selectedColumns(layout)) == 0 {
		return state
	}
	if layout == layoutQualifying && a.fieldVisible(layout, "gap") {
		state.SessionBest = sessionBest(a.frame)
	}
	if layout == layoutRace || !a.fieldVisible(layout, "setup") {
		state.Expanded = ""
	}
	var storage [maxCars]Car
	for n, c := range visibleInto(a.frame, a.mode, storage[:0]) {
		c.Flags &= inPitLane | resultMask
		c.Known &= knownName | knownPosition | knownGrid | knownTyre | knownLast | knownBest | knownPit | knownResult | knownLaps | knownGapAhead | knownSectors | knownStint
		if !badgeRowReady(c) {
			c.BadgePNG = ""
			c.TyrePNG = ""
		}
		if !a.fieldVisible(layout, "team") {
			c.BadgePNG = ""
		}
		if !a.fieldVisible(layout, "tyre") {
			c.TyrePNG = ""
		}
		if c.Known&knownResult == 0 {
			c.Flags &^= resultMask
		}
		if c.Known&knownPit == 0 {
			c.Flags &^= inPitLane
		}
		if c.Known&knownName == 0 {
			c.Name = ""
		}
		if c.Known&knownPosition == 0 {
			c.Position = 0
		}
		if c.Known&knownGrid == 0 {
			c.Grid = 0
		}
		if c.Known&knownTyre == 0 {
			c.Compound = 0
			c.Visual = 0
		}
		if c.Known&knownLast == 0 {
			c.Last = 0
		}
		if c.Known&knownBest == 0 {
			c.Best = 0
		}
		if layout != layoutRace {
			c.WearKnown = 0
			c.Wear = [4]float32{}
			c.Known &^= knownGapAhead | knownSectors
			c.GapAhead = 0
			c.BestSectors = [3]uint32{}
			if c.Known&knownPosition != 0 {
				c.Known &^= knownGrid
				c.Grid = 0
			}
		} else {
			c.Known &^= knownLaps
			c.Laps = 0
		}
		if c.Known&knownLaps == 0 {
			c.Laps = 0
		}
		if c.Known&knownGapAhead == 0 || retirementLabel(c) != "" {
			c.Known &^= knownGapAhead
			c.GapAhead = 0
		}
		if c.Known&knownSectors == 0 {
			c.BestSectors = [3]uint32{}
		}
		if layout != layoutRace || !a.fieldVisible(layout, "stint") {
			c.Known &^= knownStint
		}
		if c.Known&knownStint == 0 {
			c.StintLaps = 0
		}
		c.WearKnown &= 15
		for wheel := range c.Wear {
			if c.WearKnown&(1<<wheel) == 0 {
				c.Wear[wheel] = 0
			}
		}
		if layout == layoutRace || !a.fieldVisible(layout, "setup") || a.expanded == "" || a.expanded != rowID(a.frame, c) {
			c.SetupKnown = 0
		}
		c.SetupKnown &= (1 << 21) - 1
		for setting := range c.Setup {
			if c.SetupKnown&(1<<setting) == 0 {
				c.Setup[setting] = 0
			}
		}
		if !a.fieldVisible(layout, "last") {
			c.Known &^= knownLast
			c.Last = 0
		}
		if !a.fieldVisible(layout, "laps") {
			c.Known &^= knownLaps
			c.Laps = 0
		}
		if !a.fieldVisible(layout, "gap") {
			c.Known &^= knownGapAhead
			c.GapAhead = 0
		}
		if !a.fieldVisible(layout, "best") && !(layout == layoutQualifying && a.fieldVisible(layout, "gap")) {
			c.Known &^= knownBest
			c.Best = 0
		}
		if !a.fieldVisible(layout, "S1") && !a.fieldVisible(layout, "S2") && !a.fieldVisible(layout, "S3") {
			c.Known &^= knownSectors
			c.BestSectors = [3]uint32{}
		}
		if layout == layoutRace && !a.fieldVisible(layout, "Avg") {
			for i, key := range []string{"RL", "RR", "FL", "FR"} {
				if !a.fieldVisible(layout, key) {
					c.WearKnown &^= 1 << i
					c.Wear[i] = 0
				}
			}
		}
		state.Cars[n] = c
		state.Count++
	}
	return state
}

func (a *App) needsView() bool { return !a.viewSent || a.viewState() != a.lastView }

// Typed cells preserve the launcher JSON while avoiding several maps per cell.
type textValue struct {
	Text string `json:"text"`
	Tone string `json:"tone"`
}
type wearValue struct {
	Text string  `json:"text"`
	Sort float32 `json:"sort"`
	Tone string  `json:"tone"`
}
type toggleField struct {
	Value bool `json:"value"`
}
type toggleEdit struct {
	Toggle toggleField `json:"toggle"`
}
type toggleValue struct {
	Text string     `json:"text"`
	Edit toggleEdit `json:"edit"`
}

func tyreCell(c Car) any {
	if c.TyrePNG == "" || !badgeRowReady(c) {
		return "—"
	}
	return M{"image_data": M{"png": c.TyrePNG, "width": 24, "height": 24, "fit": "contain", "transparent": true}, "sort": c.Compound}
}

func wearCell(value float32, known bool) any {
	if !known {
		return textValue{Text: "—", Tone: "muted"}
	}
	tone := "good"
	if value >= 50 {
		tone = "warn"
	}
	if value >= 75 {
		tone = "bad"
	}
	return wearValue{Text: fmt.Sprintf("%.1f%%", value), Sort: value, Tone: tone}
}

var setupNames = [21]string{"Front wing", "Rear wing", "Differential · on throttle", "Differential · off throttle", "Front camber", "Rear camber", "Front toe", "Rear toe", "Front suspension", "Rear suspension", "Front anti-roll bar", "Rear anti-roll bar", "Front ride height", "Rear ride height", "Brake pressure", "Front brake bias", "Front-left pressure", "Front-right pressure", "Rear-left pressure", "Rear-right pressure", "Fuel remaining"}

func setupValue(c Car, key int) string {
	if c.SetupKnown&(1<<key) == 0 {
		return "—"
	}
	v := c.Setup[key]
	switch {
	case key == 2 || key == 3 || key == 14 || key == 15:
		return fmt.Sprintf("%.0f%%", v)
	case key >= 4 && key <= 7:
		return fmt.Sprintf("%.2f°", v)
	case key >= 16 && key <= 19:
		return fmt.Sprintf("%.1f psi", v)
	case key == 20:
		return fmt.Sprintf("%.1f kg", v)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}
func setupDetail(c Car) []M {
	rows := [][]string{}
	for key := 0; key < 21; key += 2 {
		row := []string{setupNames[key], setupValue(c, key), "", ""}
		if key+1 < 21 {
			row[2] = setupNames[key+1]
			row[3] = setupValue(c, key+1)
		}
		rows = append(rows, row)
	}
	return []M{{"table": M{"id": fmt.Sprintf("setup-values-%d", c.Vehicle), "columns": []M{{"key": "left-setting", "title": "Setting"}, {"key": "left-value", "title": "Value", "width": 90}, {"key": "right-setting", "title": "Setting"}, {"key": "right-value", "title": "Value", "width": 90}}, "rows": rows, "compact": true}}}
}
func (a *App) view() M {
	layout := layoutFor(a.frame)
	selected := a.selectedColumns(layout)
	if a.disabled[layout] || len(selected) == 0 {
		message := "All columns are hidden. Use Columns to choose what to show."
		actions := []M{{"id": "columns", "label": "Columns", "style": "ghost"}}
		if a.disabled[layout] {
			message = "Disabled for this session. Enable " + layoutNames[layout] + " in the add-on settings."
			actions = []M{}
		}
		return a.withColumns(M{"subtitle": a.pageSubtitle(), "tick": true, "actions": actions, "body": M{"blocks": []M{textBlock(message)}}}, layout)
	}
	best := sessionBest(a.frame)
	blocks := []M{}
	batch := []M{}
	suffix := ""
	flush := func() {
		if len(batch) > 0 {
			blocks = append(blocks, M{"table": M{"id": "drivers" + suffix, "columns": selected, "rows": batch, "compact": true}})
			batch = []M{}
		}
	}
	for _, c := range visible(a.frame, a.mode) {
		open := layout != layoutRace && a.fieldVisible(layout, "setup") && a.expanded == rowID(a.frame, c)
		cells := driverCells(c, layout, best, open)
		row := M{"id": rowID(a.frame, c), "cells": cells}
		if retirementLabel(c) != "" {
			row["tone"] = "muted"
			for i, cell := range cells {
				if wear, ok := cell.(wearValue); ok {
					wear.Tone = "muted"
					cells[i] = wear
				}
				if value, ok := cell.(M); ok {
					if _, exists := value["tone"]; exists {
						value["tone"] = "muted"
					}
				}
			}
		} else if a.highlight && int32(c.Vehicle) == a.frame.Player {
			row["tone"] = "accent"
		}
		row["cells"] = a.selectedCells(layout, cells)
		batch = append(batch, row)
		if open {
			flush()
			blocks = append(blocks, M{"card": M{"title": "Car setup · " + driverName(c), "blocks": setupDetail(c)}})
			suffix = fmt.Sprintf("-after-%d", c.Vehicle)
		}
	}
	flush()
	if len(blocks) == 0 {
		message := a.status
		if a.frame.State&active != 0 && a.mode != "full" {
			message = "Waiting to identify your car. Full grid is available in the add-on settings."
		}
		blocks = append(blocks, M{"table": M{"id": "drivers", "columns": selected, "rows": []M{}, "compact": true, "empty": message}})
	}
	if a.frame.Diagnostic != "" {
		blocks = append(blocks, textBlock(a.frame.Diagnostic))
	}
	return a.withColumns(M{"subtitle": a.pageSubtitle(), "tick": true, "actions": []M{{"id": "columns", "label": "Columns", "style": "ghost"}}, "body": M{"blocks": blocks}}, layout)
}
func (a *App) pageSubtitle() string {
	subtitle := a.status
	frame := a.frame
	if !a.fieldVisible(layoutFor(frame), "counter") {
		frame.State &^= sessionTimer | sessionLaps
	}
	if summary := sessionSummary(frame); summary != "" {
		subtitle += " · " + summary
	}
	return subtitle + " · v" + addonVersion
}
