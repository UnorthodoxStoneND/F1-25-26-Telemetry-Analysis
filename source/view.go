package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
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
		if surnameBytes := int((c.Flags >> 16) & 255); surnameBytes > 0 && surnameBytes < len(c.Name) && c.Name[surnameBytes] == ' ' {
			return strings.ToUpper(c.Name[:surnameBytes]) + c.Name[surnameBytes:]
		}
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
func driverCell(c Car) any {
	name := driverName(c)
	if label := retirementLabel(c); label != "" {
		return M{"text": name + " · " + label, "spans": []M{{"text": name + " · "}, {"text": label, "bold": true}}}
	}
	if c.Known&knownPit != 0 && c.Flags&inPitLane != 0 {
		return M{"text": name + " · PIT", "spans": []M{{"text": name + " · "}, {"text": "PIT", "tone": "warn", "bold": true}}}
	}
	return name
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
	Session                            uint64
	Player                             int32
	Status, Diagnostic, Mode, Expanded string
	Highlight, Active                  bool
	Count                              int
	Cars                               [maxCars]Car
}

func (a *App) viewState() viewState {
	state := viewState{Session: a.frame.Session, Player: a.frame.Player,
		Status: a.status, Diagnostic: a.frame.Diagnostic, Mode: a.mode,
		Expanded: a.expanded, Highlight: a.highlight, Active: a.frame.State&active != 0}
	var storage [maxCars]Car
	for n, c := range visibleInto(a.frame, a.mode, storage[:0]) {
		c.Flags &= (255 << 16) | inPitLane | resultMask
		c.Known &= knownName | knownPosition | knownGrid | knownTyre | knownLast | knownBest | knownPit | knownResult
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
		c.WearKnown &= 15
		for wheel := range c.Wear {
			if c.WearKnown&(1<<wheel) == 0 {
				c.Wear[wheel] = 0
			}
		}
		if a.expanded == "" || a.expanded != rowID(a.frame, c) {
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
	Bar  float64 `json:"bar"`
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
	return wearValue{Text: fmt.Sprintf("%.1f%%", value), Bar: float64(value) / 100, Sort: value, Tone: tone}
}

var driverColumns = makeColumns()

func columns() []M { return driverColumns }
func makeColumns() []M {
	c := []M{{"key": "position", "title": "Pos", "width": 46}, {"key": "driver", "title": "Driver"}, {"key": "tyre", "title": "Tyre", "width": 68}}
	for _, s := range []string{"FL", "FR", "RL", "RR", "Avg"} {
		c = append(c, M{"key": s, "title": s})
	}
	return append(c, M{"key": "last", "title": "Last lap"}, M{"key": "best", "title": "Best lap"}, M{"key": "setup", "title": "Setup", "width": 64})
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
	blocks := []M{}
	batch := []M{}
	suffix := ""
	flush := func() {
		if len(batch) > 0 {
			blocks = append(blocks, M{"table": M{"id": "drivers" + suffix, "columns": columns(), "rows": batch, "compact": true}})
			batch = []M{}
		}
	}
	for _, c := range visible(a.frame, a.mode) {
		p := "—"
		if position(c) < 1000 {
			p = fmt.Sprint(position(c))
		}
		open := a.expanded == rowID(a.frame, c)
		cells := []any{p, driverCell(c), tyreCell(c)}
		for _, wheel := range []int{2, 3, 0, 1} {
			cells = append(cells, wearCell(c.Wear[wheel], c.WearKnown&(1<<wheel) != 0))
		}
		avg := (c.Wear[0] + c.Wear[1] + c.Wear[2] + c.Wear[3]) / 4
		cells = append(cells, wearCell(avg, c.WearKnown&15 == 15), lapTime(c.Last, c.Known&knownLast != 0), lapTime(c.Best, c.Known&knownBest != 0), toggleValue{Edit: toggleEdit{Toggle: toggleField{Value: open}}})
		row := M{"id": rowID(a.frame, c), "cells": cells}
		if retirementLabel(c) != "" {
			row["tone"] = "muted"
			for i, cell := range cells {
				if wear, ok := cell.(wearValue); ok {
					wear.Tone = "muted"
					cells[i] = wear
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
		blocks = append(blocks, M{"table": M{"id": "drivers", "columns": columns(), "rows": []M{}, "compact": true, "empty": message}})
	}
	if a.frame.Diagnostic != "" {
		blocks = append(blocks, textBlock(a.frame.Diagnostic))
	}
	return M{"subtitle": a.status + " · v" + addonVersion, "tick": true, "body": M{"blocks": blocks}}
}
