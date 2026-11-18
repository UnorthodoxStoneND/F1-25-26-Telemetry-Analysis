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
	state := viewState{Layout: layout, Session: a.frame.Session, Player: a.frame.Player,
		SessionLabel: sessionSummary(a.frame),
		Status:       a.status, Diagnostic: a.frame.Diagnostic, Mode: a.mode,
		Expanded: a.expanded, Highlight: a.highlight, Active: a.frame.State&active != 0}
	if layout == layoutQualifying {
		state.SessionBest = sessionBest(a.frame)
	}
	if layout == layoutRace {
		state.Expanded = ""
	}
	var storage [maxCars]Car
	for n, c := range visibleInto(a.frame, a.mode, storage[:0]) {
		c.Flags &= inPitLane | resultMask
		c.Known &= knownName | knownPosition | knownGrid | knownTyre | knownLast | knownBest | knownPit | knownResult | knownLaps | knownGapAhead | knownSectors
		if !badgeRowReady(c) {
			c.BadgePNG = ""
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
		c.WearKnown &= 15
		for wheel := range c.Wear {
			if c.WearKnown&(1<<wheel) == 0 {
				c.Wear[wheel] = 0
			}
		}
		if layout == layoutRace || a.expanded == "" || a.expanded != rowID(a.frame, c) {
			c.SetupKnown = 0
		}
		c.SetupKnown &= (1 << 21) - 1
		for setting := range c.Setup {
			if c.SetupKnown&(1<<setting) == 0 {
				c.Setup[setting] = 0
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
type colourField struct {
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}
type colourEdit struct {
	Colour colourField `json:"colour"`
}
type tyreValue struct {
	Text string     `json:"text"`
	Sort string     `json:"sort"`
	Edit colourEdit `json:"edit"`
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
	if c.Known&knownTyre == 0 {
		return "—"
	}
	labels := map[int32]string{16: "C5", 17: "C4", 18: "C3", 19: "C2", 20: "C1", 7: "INT", 8: "WET"}
	label := labels[c.Compound]
	if label == "" {
		return "—"
	}
	colour := ""
	switch c.Visual {
	case 16:
		colour = "#FF4A5B"
	case 17:
		colour = "#FFD53F"
	case 18:
		colour = "#EEF2FA"
	}
	if c.Compound == 7 {
		colour = "#41DA8B"
	}
	if c.Compound == 8 {
		colour = "#4B9FFF"
	}
	if colour == "" {
		return label
	} // Never guess a dry tyre's colour from its absolute compound.
	return tyreValue{Text: label, Sort: label, Edit: colourEdit{Colour: colourField{Value: colour, Disabled: true}}}
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
	best := sessionBest(a.frame)
	blocks := []M{}
	batch := []M{}
	suffix := ""
	flush := func() {
		if len(batch) > 0 {
			blocks = append(blocks, M{"table": M{"id": "drivers" + suffix, "columns": columns(layout), "rows": batch, "compact": true}})
			batch = []M{}
		}
	}
	for _, c := range visible(a.frame, a.mode) {
		open := layout != layoutRace && a.expanded == rowID(a.frame, c)
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
		blocks = append(blocks, M{"table": M{"id": "drivers", "columns": columns(layout), "rows": []M{}, "compact": true, "empty": message}})
	}
	if a.frame.Diagnostic != "" {
		blocks = append(blocks, textBlock(a.frame.Diagnostic))
	}
	subtitle := a.status
	if summary := sessionSummary(a.frame); summary != "" {
		subtitle += " · " + summary
	}
	return M{"subtitle": subtitle + " · v" + addonVersion, "tick": true, "body": M{"blocks": blocks}}
}
