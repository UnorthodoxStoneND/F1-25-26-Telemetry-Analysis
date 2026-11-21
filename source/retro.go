package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// The GUI receives immutable presentation values, never mapped memory or App.
type retroCell struct{ Text, PNG, Tone string }
type retroColumn struct {
	Key, Title string
	Weight     float32
	Width      float32
}
type retroRow struct {
	ID                      string
	Player, Retired, AtRisk bool
	Cells                   []retroCell
}
type retroModel struct {
	Show, ColumnsOpen             bool
	DisableSlides                 bool
	DataDir                       string
	PreferenceKey                 string
	Title, Status, Message, Error string
	Layout                        sessionLayout
	Columns                       []retroColumn
	Choices                       []retroChoice
	Rows                          []retroRow
	SetupTitle                    string
	Setup                         []string
}
type retroChoice struct {
	Key, Label string
	Visible    bool
}
type desktopAction struct {
	Kind, Key, Row string
	Layout         sessionLayout
	Visible        bool
	Width          float32
	PreferenceKey  string
}
type desktopBridge struct {
	models  chan retroModel
	actions chan desktopAction
}

func newDesktopBridge() *desktopBridge {
	return &desktopBridge{make(chan retroModel, 1), make(chan desktopAction, 64)}
}
func (b *desktopBridge) publish(m retroModel) {
	select {
	case b.models <- m:
		return
	default:
	}
	select {
	case <-b.models:
	default:
	}
	select {
	case b.models <- m:
	default:
	}
}
func (b *desktopBridge) act(a desktopAction) {
	select {
	case b.actions <- a:
	default:
	}
}

// A second advancing publisher sample is required before opening. This avoids
// opening a stale Linux IPC file left by a crashed game. Pause ticks continue.
type windowLifecycle struct {
	session, tick    uint64
	lastChange       time.Time
	fresh, dismissed bool
}

func (l *windowLifecycle) dismiss() { l.dismissed = true }
func (l *windowLifecycle) observe(selected bool, f Frame, now time.Time) bool {
	if !selected || f.State&active == 0 || f.State&windowReady == 0 {
		*l = windowLifecycle{}
		return false
	}
	if l.session != f.Session || l.lastChange.IsZero() {
		*l = windowLifecycle{session: f.Session, tick: f.Tick, lastChange: now}
	} else if l.tick != f.Tick {
		l.tick = f.Tick
		l.lastChange = now
		l.fresh = true
	}
	return l.fresh && !l.dismissed && now.Sub(l.lastChange) < 3*time.Second
}
func retroWeight(key string) float32 {
	switch key {
	case "position", "gain", "team":
		return 2.4
	case "driver":
		return 11
	case "tyre":
		return 3.7
	case "last", "best":
		return 8
	case "gap":
		return 7.5
	case "tyre_age":
		return 6
	case "pen":
		return 5
	case "status":
		return 5
	case "setup":
		return 5
	case "S1", "S2", "S3":
		return 6
	case "laps":
		return 4
	default:
		return 5
	}
}
func retroValue(value any) retroCell {
	switch v := value.(type) {
	case string:
		return retroCell{Text: v}
	case textValue:
		return retroCell{Text: v.Text, Tone: v.Tone}
	case wearValue:
		return retroCell{Text: v.Text, Tone: v.Tone}
	case toggleValue:
		if v.Edit.Toggle.Value {
			return retroCell{Text: "CLOSE"}
		}
		return retroCell{Text: "VIEW"}
	case M:
		if art, ok := v["image_data"].(M); ok {
			png, _ := art["png"].(string)
			return retroCell{PNG: png}
		}
		text, _ := v["text"].(string)
		tone, _ := v["tone"].(string)
		return retroCell{Text: text, Tone: tone}
	}
	return retroCell{Text: "—"}
}
func (a *App) retroModel(show bool) retroModel {
	layout := layoutFor(a.frame)
	frame := a.frame
	if !a.fieldVisible(layout, "counter") {
		frame.State &^= sessionTimer | sessionLaps
	}
	m := retroModel{DataDir: a.data, PreferenceKey: a.preferencePath(), Show: show, DisableSlides: a.disableSlides, Layout: layout, Title: sessionSummary(frame), Status: a.status + " · v" + addonVersion, ColumnsOpen: a.columnsOpen && a.editorLayout == layout, Error: a.frame.Diagnostic}
	m.Error = strings.TrimSpace(m.Error + " " + a.desktopError)
	if a.disabled[layout] {
		m.Message = "Disabled for this session. Enable " + layoutNames[layout] + " in the add-on settings."
		return m
	}
	label := "Time remaining"
	if layout == layoutRace {
		label = "Lap counter"
	}
	m.Choices = append(m.Choices, retroChoice{"counter", label, a.fieldVisible(layout, "counter")})
	for _, col := range columns(layout) {
		key, title := col["key"].(string), col["title"].(string)
		if key == "team" {
			title = "Team"
		}
		m.Choices = append(m.Choices, retroChoice{key, title, a.fieldVisible(layout, key)})
		if a.fieldVisible(layout, key) {
			m.Columns = append(m.Columns, retroColumn{Key: key, Title: title, Weight: retroWeight(key), Width: a.widths[layout][key]})
		}
	}
	if len(m.Columns) == 0 {
		m.Message = "All columns are hidden. Use COLUMNS to choose what to show."
		return m
	}
	best := sessionBest(a.frame)
	for _, c := range visible(a.frame, a.mode) {
		open := a.expanded == rowID(a.frame, c) && layout != layoutRace && a.fieldVisible(layout, "setup")
		row := retroRow{ID: rowID(a.frame, c), Player: a.highlight && int32(c.Vehicle) == a.frame.Player, Retired: retirementLabel(c) != "", AtRisk: qualifyingAtRisk(a.frame, c)}
		for _, cell := range a.selectedCells(layout, driverCells(c, layout, best, open)) {
			row.Cells = append(row.Cells, retroValue(cell))
		}
		// Mark the current session best exactly as the reference's purple time.
		if !row.Retired && c.Known&knownBest != 0 && c.Best > 0 && c.Best == best {
			for i, col := range m.Columns {
				if col.Key == "best" {
					row.Cells[i].Tone = "fastest"
				}
			}
		}
		m.Rows = append(m.Rows, row)
		if open {
			m.SetupTitle = "CAR SETUP · " + driverName(c)
			for i, name := range setupNames {
				m.Setup = append(m.Setup, fmt.Sprintf("%s  %s", name, setupValue(c, i)))
			}
		}
	}
	if len(m.Rows) == 0 {
		m.Message = "Waiting to identify your car. Full grid is available in the add-on settings."
	}
	return m
}
func (a *App) retroAction(action desktopAction) {
	layout := layoutFor(a.frame)
	if action.Layout != layout || a.disabled[layout] || (action.PreferenceKey != "" && action.PreferenceKey != a.preferencePath()) {
		return
	}
	switch action.Kind {
	case "width":
		if fieldBit(layout, action.Key) == 0 || action.Key == "counter" || (action.Width != 0 && !validColumnWidth(action.Width)) {
			return
		}
		next := copyColumnWidths(a.widths)
		if next[layout] == nil {
			next[layout] = map[string]float32{}
		}
		if action.Width == 0 {
			delete(next[layout], action.Key)
		} else {
			next[layout][action.Key] = action.Width
		}
		if err := a.saveColumnState(a.hidden, next); err != nil {
			a.desktopError = err.Error()
		} else {
			a.desktopError = ""
		}
	case "columns":
		a.columnsOpen = !a.columnsOpen
		a.editorLayout = layout
	case "field":
		next := a.hidden
		bit := fieldBit(layout, action.Key)
		if action.Visible {
			next[layout] &^= bit
		} else {
			next[layout] |= bit
		}
		if err := a.saveColumns(next); err != nil {
			a.desktopError = err.Error()
		} else {
			a.desktopError = ""
		}
	case "reset":
		next := a.hidden
		next[layout] = 0
		if err := a.saveColumnState(next, resetColumnWidths(a.widths, layout)); err != nil {
			a.desktopError = err.Error()
		} else {
			a.desktopError = ""
		}
	case "setup":
		values := map[string]json.RawMessage{"setup": json.RawMessage("true")}
		if a.expanded == action.Row {
			values["setup"] = json.RawMessage("false")
		}
		raw, _ := json.Marshal(M{"event": M{"submit": M{"form": "drivers", "row": action.Row, "values": values}}})
		_, _ = a.event(raw)
	}
}
func retroImageBytes(encoded string) []byte {
	b, _ := base64.StdEncoding.DecodeString(encoded)
	return b
}
