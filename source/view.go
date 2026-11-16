package main

import (
	"fmt"
	"sort"
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
	cars := append([]Car(nil), f.Cars...)
	sort.SliceStable(cars, func(i, j int) bool { return position(cars[i]) < position(cars[j]) })
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
	return M{"text": label, "sort": label, "edit": M{"colour": M{"value": colour, "disabled": true}}}
}
func wearCell(value float32, known bool) M {
	if !known {
		return M{"text": "—", "tone": "muted"}
	}
	tone := "good"
	if value >= 50 {
		tone = "warn"
	}
	if value >= 75 {
		tone = "bad"
	}
	return M{"text": fmt.Sprintf("%.1f%%", value), "bar": float64(value) / 100, "sort": value, "tone": tone}
}
func columns() []M {
	c := []M{{"key": "position", "title": "Pos", "width": 46}, {"key": "driver", "title": "Driver"}, {"key": "tyre", "title": "Tyre", "width": 68}}
	for _, s := range []string{"FL", "FR", "RL", "RR", "Avg"} {
		c = append(c, M{"key": s, "title": s, "width": 84})
	}
	return append(c, M{"key": "last", "title": "Last lap", "width": 98}, M{"key": "best", "title": "Best lap", "width": 98}, M{"key": "setup", "title": "Setup", "width": 64})
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
		cells := []any{p, driverName(c), tyreCell(c)}
		for _, wheel := range []int{2, 3, 0, 1} {
			cells = append(cells, wearCell(c.Wear[wheel], c.WearKnown&(1<<wheel) != 0))
		}
		avg := (c.Wear[0] + c.Wear[1] + c.Wear[2] + c.Wear[3]) / 4
		cells = append(cells, wearCell(avg, c.WearKnown&15 == 15), lapTime(c.Last, c.Known&knownLast != 0), lapTime(c.Best, c.Known&knownBest != 0), M{"text": "", "edit": M{"toggle": M{"value": open}}})
		row := M{"id": rowID(a.frame, c), "cells": cells}
		if a.highlight && int32(c.Vehicle) == a.frame.Player {
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
