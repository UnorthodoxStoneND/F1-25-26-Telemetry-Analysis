package main

import (
	"fmt"
	"time"
)

type M = map[string]any

func textBlock(s string) M       { return M{"text": M{"text": s, "tone": "neutral"}} }
func facts(items ...M) M         { return M{"facts": items} }
func fact(label, value string) M { return M{"label": label, "value": value} }
func fmtTime(ms uint32) string   { return fmt.Sprintf("%d:%02d.%03d", ms/60000, ms/1000%60, ms%1000) }
func compound(id int) string {
	if s, ok := map[int]string{16: "C5", 17: "C4", 18: "C3", 19: "C2", 20: "C1", 21: "C0", 22: "C6", 7: "INT", 8: "WET"}[id]; ok {
		return s
	}
	return "—"
}
func tyreColour(c Car) string {
	if c.Compound == 7 {
		return "green"
	}
	if c.Compound == 8 {
		return "blue"
	}
	switch c.Visual {
	case 16:
		return "red"
	case 17:
		return "yellow"
	case 18:
		return "white"
	}
	return ""
}
func sessionName(s int) string {
	switch {
	case s >= 1 && s <= 4:
		return "Practice"
	case s >= 5 && s <= 9:
		return "Qualifying"
	case s >= 10 && s <= 14:
		return "Sprint qualifying"
	case s >= 15 && s <= 17:
		return "Race"
	case s == 18:
		return "Time Trial"
	}
	return "Session"
}
func position(c Car) int {
	if c.Position > 0 {
		return c.Position
	}
	return c.Grid
}
func driverName(i int, c Car) string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("Driver %d", i+1)
}
func restricted(i, player int, c Car) bool { return c.Participant && !c.AI && !c.Public && i != player }
func wearText(c Car, w int, hidden bool) string {
	if !c.HasWear || hidden {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", c.Wear[w])
}
func wearCell(value float64, known bool) M {
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
	return M{"text": fmt.Sprintf("%.1f%%", value), "bar": value / 100, "sort": value, "tone": tone}
}

// Compact paired values are placed beneath the selected timing row, never
// in the launcher's native table detail panel (which can open at the side).
func setupDetail(c Car) []M {
	if len(c.Setup) != 50 {
		return []M{textBlock("Setup unavailable — waiting for the game's Car Setups packet.")}
	}
	r := c.Setup
	b := func(off int, unit string) string { return fmt.Sprintf("%d%s", r[off], unit) }
	f := func(off int, unit string) string {
		v := f32(r[off:])
		if !finite(v) {
			return "—"
		}
		return fmt.Sprintf("%.2f%s", v, unit)
	}
	rows := [][]string{
		{"Front wing", b(0, ""), "Rear wing", b(1, "")},
		{"Differential · on throttle", b(2, "%"), "Differential · off throttle", b(3, "%")},
		{"Front camber", f(4, "°"), "Rear camber", f(8, "°")},
		{"Front toe", f(12, "°"), "Rear toe", f(16, "°")},
		{"Front suspension", b(20, ""), "Rear suspension", b(21, "")},
		{"Front anti-roll bar", b(22, ""), "Rear anti-roll bar", b(23, "")},
		{"Front ride height", b(24, ""), "Rear ride height", b(25, "")},
		{"Brake pressure", b(26, "%"), "Front brake bias", b(27, "%")},
		{"Front-left pressure", f(37, " psi"), "Front-right pressure", f(41, " psi")},
		{"Rear-left pressure", f(29, " psi"), "Rear-right pressure", f(33, " psi")},
		{"Fuel load", f(46, " kg"), "", ""},
	}
	return []M{{"table": M{"id": "setup-values", "columns": []M{{"key": "left-setting", "title": "Setting"}, {"key": "left-value", "title": "Value", "width": 90}, {"key": "right-setting", "title": "Setting"}, {"key": "right-value", "title": "Value", "width": 90}}, "rows": rows, "compact": true}}}
}
func timingColumns() []M {
	cols := []M{{"key": "position", "title": "Pos", "width": 46}, {"key": "driver", "title": "Driver"}, {"key": "tyre", "title": "Tyre", "width": 68}}
	for _, name := range []string{"FL", "FR", "RL", "RR", "Avg"} {
		cols = append(cols, M{"key": name, "title": name, "width": 84})
	}
	return append(cols, M{"key": "last", "title": "Last lap", "width": 98}, M{"key": "best", "title": "Best lap", "width": 98}, M{"key": "setup", "title": "Setup", "width": 64})
}
func (a *App) telemetryView() M {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := &a.tel
	status := "Waiting for UDP"
	switch {
	case !a.enabled:
		status = "Addon disabled"
	case a.listenErr != "":
		status = "UDP listener unavailable"
	case a.gameKnown && !a.gameRunning && !t.Active:
		status = "Game stopped"
	case t.Ended:
		status = "Session ended"
	case t.Active && t.Paused:
		status = "Paused · values retained"
	case t.Active && time.Since(t.Last) > 3*time.Second:
		status = "No new packets · values retained"
	case t.Active:
		status = "Live"
	}
	blocks := []M{}
	if a.listenErr != "" {
		blocks = append(blocks, textBlock(a.listenErr), M{"buttons": []M{{"id": "retry", "label": "Retry UDP listener", "icon": "refresh"}}})
	}
	batch := []M{}
	flush := func(suffix string) {
		if len(batch) == 0 {
			return
		}
		blocks = append(blocks, M{"table": M{"id": "drivers" + suffix, "columns": timingColumns(), "rows": batch, "compact": true}})
		batch = []M{}
	}
	visible := t.visible(a.prefs.Mode)
	suffix := ""
	for _, i := range visible {
		c := t.Cars[i]
		known := c.HasWear && !restricted(i, t.Player, c)
		pos := "—"
		if position(c) > 0 {
			pos = fmt.Sprint(position(c))
		}
		pill := tyreCell(c)
		open := a.expandedSet && a.expanded == i && a.expandedUID == t.UID
		avg := (c.Wear[0] + c.Wear[1] + c.Wear[2] + c.Wear[3]) / 4
		row := M{"id": fmt.Sprintf("car-%d", i), "cells": []any{pos, driverName(i, c), pill, wearCell(c.Wear[2], known), wearCell(c.Wear[3], known), wearCell(c.Wear[0], known), wearCell(c.Wear[1], known), wearCell(avg, known), lapTime(c.Last), lapTime(c.Best), M{"text": "", "edit": M{"toggle": M{"value": open}}}}}
		if i == t.Player && a.prefs.Highlight {
			row["tone"] = "accent"
		}
		batch = append(batch, row)
		if open {
			flush(suffix)
			blocks = append(blocks, M{"card": M{"title": "Car setup · " + driverName(i, c), "blocks": setupDetail(c)}})
			suffix = fmt.Sprintf("-after-%d", i)
		}
	}
	flush(suffix)
	if a.prefs.Mode != "none" && len(visible) == 0 {
		blocks = append(blocks, M{"table": M{"id": "drivers", "columns": timingColumns(), "rows": []M{}, "compact": true, "empty": "Waiting for session participants. The page will show each field as soon as its UDP packet arrives."}})
	}
	if !t.Active {
		blocks = append(blocks, textBlock("Game telemetry: UDP On · Broadcast Off · IP 127.0.0.1 · match the port above · format 2025 or 2026 · 20 Hz."))
	}
	return M{"subtitle": fmt.Sprintf("%s · %s · UDP %d · v%s", status, sessionName(t.Session), t.Format, addonVersion), "tick": true, "body": M{"blocks": blocks}}
}
