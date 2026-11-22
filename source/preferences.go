package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var layoutKeys = [3]string{"practice", "qualifying", "race"}
var layoutNames = [3]string{"Practice", "Qualifying / Sprint Qualifying", "Sprint / Race"}

func validProfile(name string) bool {
	if name == "" || len(name) > 40 {
		return false
	}
	for _, ch := range name {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}
func fieldBit(layout sessionLayout, key string) uint32 {
	if key == "counter" {
		return 1 << 31
	}
	for i, col := range columns(layout) {
		if col["key"] == key {
			return 1 << i
		}
	}
	return 0
}
func (a *App) fieldVisible(layout sessionLayout, key string) bool {
	return a.hidden[layout]&fieldBit(layout, key) == 0
}
func (a *App) selectedColumns(layout sessionLayout) []M {
	out := []M{}
	for _, col := range columns(layout) {
		if a.fieldVisible(layout, col["key"].(string)) {
			out = append(out, col)
		}
	}
	return out
}
func (a *App) selectedCells(layout sessionLayout, cells []any) []any {
	out := []any{}
	for i, col := range columns(layout) {
		if a.fieldVisible(layout, col["key"].(string)) {
			out = append(out, cells[i])
		}
	}
	return out
}
func (a *App) preferencePath() string {
	if a.data == "" || !validProfile(a.profileName) {
		return ""
	}
	return filepath.Join(a.data, "columns-"+a.profileName+".json")
}

type columnPreferences struct {
	Version int                           `json:"version"`
	Hidden  map[string][]string           `json:"hidden"`
	Widths  map[string]map[string]float32 `json:"widths,omitempty"`
}

func (a *App) loadColumns() {
	path := a.preferencePath()
	if path == a.columnPath {
		return
	}
	a.columnPath = path
	a.hidden = [3]uint32{}
	a.widths = [3]map[string]float32{}
	a.columnsOpen = false
	if path == "" {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	var saved columnPreferences
	if json.NewDecoder(io.LimitReader(f, 16*1024)).Decode(&saved) != nil || saved.Version != 1 {
		return
	}
	for layout, key := range layoutKeys {
		for field, width := range saved.Widths[key] {
			if field == "stint" {
				field = "tyre_age"
				if _, exists := saved.Widths[key][field]; exists {
					continue
				}
			}
			if field != "counter" && fieldBit(sessionLayout(layout), field) != 0 && validColumnWidth(width) {
				if a.widths[layout] == nil {
					a.widths[layout] = map[string]float32{}
				}
				a.widths[layout][field] = width
			}
		}
		for _, field := range saved.Hidden[key] {
			if field == "stint" {
				field = "tyre_age"
			}
			a.hidden[layout] |= fieldBit(sessionLayout(layout), field)
		}
	}
}
func (a *App) saveColumns(hidden [3]uint32) error { return a.saveColumnState(hidden, a.widths) }
func (a *App) saveColumnState(hidden [3]uint32, widths [3]map[string]float32) error {
	path := a.preferencePath()
	if path == "" {
		return fmt.Errorf("Column settings cannot be saved without a valid launcher profile")
	}
	saved := columnPreferences{Version: 1, Hidden: map[string][]string{}, Widths: map[string]map[string]float32{}}
	for layout, key := range layoutKeys {
		if len(widths[layout]) > 0 {
			saved.Widths[key] = widths[layout]
		}
		for _, col := range columns(sessionLayout(layout)) {
			field := col["key"].(string)
			if hidden[layout]&fieldBit(sessionLayout(layout), field) != 0 {
				saved.Hidden[key] = append(saved.Hidden[key], field)
			}
		}
		if hidden[layout]&(1<<31) != 0 {
			saved.Hidden[key] = append(saved.Hidden[key], "counter")
		}
	}
	// Only UI preferences are persisted, never telemetry or badge pixels.
	raw, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(a.data, 0700); err != nil {
		return fmt.Errorf("Save columns: %w", err)
	}
	f, err := os.CreateTemp(a.data, ".columns-*.tmp")
	if err != nil {
		return fmt.Errorf("Save columns: %w", err)
	}
	defer os.Remove(f.Name())
	_, err = f.Write(raw)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("Save columns: %w", err)
	}
	a.hidden = hidden
	a.widths = widths
	a.columnPath = path
	if !a.fieldVisible(layoutFor(a.frame), "setup") {
		a.expanded = ""
	}
	return nil
}
func (a *App) columnsPanel(layout sessionLayout) M {
	fields := []M{}
	add := func(key, label string) {
		fields = append(fields, M{"toggle": M{"id": key, "label": label, "value": a.fieldVisible(layout, key)}})
	}
	label := "Time remaining"
	if layout == layoutRace {
		label = "Lap counter"
	}
	add("counter", label)
	for _, col := range columns(layout) {
		key := col["key"].(string)
		label := col["title"].(string)
		if key == "team" {
			label = "Team badge"
		}
		if key == "gain" {
			label = "Positions gained / lost"
		}
		add(key, label)
	}
	return M{"card": M{"title": layoutNames[layout] + " columns", "buttons": []M{{"id": "columns-reset", "label": "Reset defaults", "style": "ghost"}}, "blocks": []M{{"form": M{"id": a.columnForm(), "fields": fields, "columns": 3, "live": true}}}}}
}
func (a *App) withColumns(view M, layout sessionLayout) M {
	if a.columnsOpen && a.editorLayout == layout && !a.disabled[layout] {
		body := view["body"].(M)
		body["blocks"] = append([]M{a.columnsPanel(layout)}, body["blocks"].([]M)...)
	}
	return view
}

func (a *App) columnForm() string {
	return fmt.Sprintf("columns-%s-%d", layoutKeys[a.editorLayout], a.columnGeneration)
}
func (a *App) columnEvent(click, form string, values map[string]json.RawMessage) (bool, string, error) {
	layout := layoutFor(a.frame)
	switch click {
	case "columns":
		if a.disabled[layout] {
			return true, "none", nil
		}
		a.columnsOpen = !a.columnsOpen || a.editorLayout != layout
		if a.columnsOpen {
			a.columnGeneration++
			a.editorLayout = layout
		}
		return true, "reload", nil
	case "columns-done":
		a.columnsOpen = false
		return true, "reload", nil
	case "columns-reset":
		if !a.columnsOpen || a.editorLayout != layout {
			return true, "none", nil
		}
		next := a.hidden
		next[layout] = 0
		if err := a.saveColumnState(next, resetColumnWidths(a.widths, layout)); err != nil {
			return true, "", err
		}
		return true, "reload", nil
	}
	if strings.HasPrefix(form, "columns-") {
		if !a.columnsOpen || a.editorLayout != layout || form != a.columnForm() || a.disabled[layout] {
			return true, "none", nil
		}
		next := a.hidden
		for field, raw := range values {
			bit := fieldBit(layout, field)
			if bit == 0 {
				continue
			}
			var on bool
			if err := json.Unmarshal(raw, &on); err != nil || string(raw) == "null" {
				return true, "", fmt.Errorf("Invalid column selection")
			}
			if on {
				next[layout] &^= bit
			} else {
				next[layout] |= bit
			}
		}
		if next != a.hidden {
			if err := a.saveColumns(next); err != nil {
				return true, "", err
			}
		}
		return true, "reload", nil
	}
	return false, "", nil
}

func validColumnWidth(width float32) bool { return width >= 32 && width <= 1200 }
func copyColumnWidths(widths [3]map[string]float32) [3]map[string]float32 {
	var out [3]map[string]float32
	for layout, fields := range widths {
		if fields != nil {
			out[layout] = map[string]float32{}
			for key, w := range fields {
				out[layout][key] = w
			}
		}
	}
	return out
}

func resetColumnWidths(widths [3]map[string]float32, layout sessionLayout) [3]map[string]float32 {
	out := copyColumnWidths(widths)
	out[layout] = nil
	return out
}
