package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clickColumns(t *testing.T, a *App, id string) {
	t.Helper()
	_, err := a.event(json.RawMessage(`{"event":{"click":{"id":"` + id + `"}}}`))
	if err != nil {
		t.Fatal(err)
	}
}
func submitColumns(a *App, group, values string) (string, error) {
	return a.event(json.RawMessage(`{"event":{"submit":{"form":"columns-` + group + `-` + fmt.Sprint(a.columnGeneration) + `","values":` + values + `}}}`))
}
func TestSavedColumnsAcrossProfilesAndSessions(t *testing.T) {
	data := t.TempDir()
	a := App{frame: grid(), mode: "full", data: data, profileName: "main"}
	a.loadColumns()
	clickColumns(t, &a, "columns")
	if _, err := submitColumns(&a, "practice", `{"position":false,"team":false,"driver":false,"setup":false}`); err != nil {
		t.Fatal(err)
	}
	clickColumns(t, &a, "columns-done")
	cells := rowCells(t, &a, 0)
	for _, key := range []string{"position", "team", "driver", "setup"} {
		if _, ok := cells[key]; ok {
			t.Fatal("hidden field retained", key)
		}
	}
	b := App{frame: grid(), mode: "full", data: data, profileName: "main"}
	b.loadColumns()
	if b.hidden != a.hidden {
		t.Fatal("preferences did not survive restart")
	}
	b.frame.SessionType = 5
	if len(rowCells(t, &b, 0)) != len(columns(layoutQualifying)) {
		t.Fatal("practice settings leaked into qualifying")
	}
	b.profileName = "other"
	b.loadColumns()
	if b.hidden != [3]uint32{} {
		t.Fatal("profile leak")
	}
	b.profileName = "main"
	b.loadColumns()
	if b.hidden != a.hidden {
		t.Fatal("switch back lost choices")
	}
	raw, err := os.ReadFile(a.preferencePath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "Driver") || strings.Contains(string(raw), "Wear") {
		t.Fatal("persisted telemetry")
	}
}
func TestAllHiddenAndReset(t *testing.T) {
	a := App{frame: grid(), mode: "full", data: t.TempDir(), profileName: "main"}
	a.loadColumns()
	for _, session := range []uint32{1, 5, 10, 15, 16} {
		a.frame.SessionType = session
		layout := layoutFor(a.frame)
		clickColumns(t, &a, "columns")
		values := map[string]bool{"counter": false}
		for _, col := range columns(layout) {
			values[col["key"].(string)] = false
		}
		raw, _ := json.Marshal(values)
		if _, err := submitColumns(&a, layoutKeys[layout], string(raw)); err != nil {
			t.Fatal(err)
		}
		clickColumns(t, &a, "columns-done")
		raw, _ = json.Marshal(a.view())
		if !strings.Contains(string(raw), "All columns are hidden") {
			t.Fatal("all hidden not recoverable", string(raw))
		}
		if len(a.view()["actions"].([]M)) != 1 {
			t.Fatal("no way to reopen selector")
		}
		clickColumns(t, &a, "columns")
		clickColumns(t, &a, "columns-reset")
		clickColumns(t, &a, "columns-done")
		if len(rowCells(t, &a, 0)) != len(columns(layout)) {
			t.Fatal("reset incomplete")
		}
	}
}
func TestColumnsRejectStaleEventsAndFailedSaves(t *testing.T) {
	a := App{frame: grid(), data: t.TempDir(), profileName: "main"}
	a.loadColumns()
	clickColumns(t, &a, "columns")
	a.frame.SessionType = 5
	if effect, err := submitColumns(&a, "practice", `{"driver":false}`); err != nil || effect != "none" || a.hidden != [3]uint32{} {
		t.Fatal("stale group submit accepted")
	}
	clickColumns(t, &a, "columns")
	if _, err := submitColumns(&a, "qualifying", `{"driver":null}`); err == nil {
		t.Fatal("null toggle accepted")
	}
	// A file in place of the data directory makes saving fail deterministically.
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	a.data = blocked
	if _, err := submitColumns(&a, "qualifying", `{"driver":false}`); err == nil || a.hidden != [3]uint32{} {
		t.Fatal("failed save applied unsaved choice")
	}
	a.profile(json.RawMessage(`{"name":"other","enabled":true}`))
	if a.columnsOpen {
		t.Fatal("profile switch left selector open")
	}
	if _, err := submitColumns(&a, "qualifying", `{"driver":false}`); err != nil || a.hidden != [3]uint32{} {
		t.Fatal("stale profile submit accepted")
	}
}
func TestPreferenceValidationAndSessionSettings(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "addon-data", "telemetry")
	if err := os.MkdirAll(filepath.Join(root, "profiles"), 0700); err != nil {
		t.Fatal(err)
	}
	raw := `{"options":{"` + addonID + `":{"show_practice":false,"show_qualifying":true,"show_race":false}}}`
	if err := os.WriteFile(filepath.Join(root, "profiles/main.json"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	a := App{frame: grid(), mode: "full", data: data, profileName: "main"}
	a.syncOptions()
	if a.disabled != [3]bool{true, false, true} {
		t.Fatal("session switches not read", a.disabled)
	}
	for _, session := range []uint32{1, 5, 10, 15, 16} {
		a.frame.SessionType = session
		view, _ := json.Marshal(a.view())
		disabled := a.disabled[layoutFor(a.frame)]
		if strings.Contains(string(view), "Disabled for this session") != disabled {
			t.Fatal("wrong disabled behaviour", session)
		}
		if disabled {
			a.lastView, a.viewSent = a.viewState(), true
			a.frame.Cars[0].Last++
			if a.needsView() {
				t.Fatal("disabled table refreshed for hidden data")
			}
		}
	}
	a.profileName = "../escape"
	a.loadColumns()
	if a.preferencePath() != "" {
		t.Fatal("profile traversal accepted")
	}
	a.profileName = "main"
	a.loadColumns()
	if err := os.MkdirAll(data, 0700); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`{`, `{"version":9,"hidden":{"practice":["driver"]}}`} {
		if err := os.WriteFile(a.preferencePath(), []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		a.columnPath = ""
		a.loadColumns()
		if a.hidden != [3]uint32{} {
			t.Fatal("invalid file changed defaults")
		}
	}
}
func TestHiddenCounterAndSetupAndLateEvents(t *testing.T) {
	a := App{frame: grid(), mode: "full"}
	a.frame.SessionType = 1
	a.frame.State |= sessionTimer
	a.frame.Progress = 125
	a.expanded = rowID(a.frame, a.frame.Cars[0])
	a.hidden[0] = fieldBit(0, "setup") | fieldBit(0, "counter")
	raw, _ := json.Marshal(a.view())
	if strings.Contains(string(raw), "remaining") || strings.Contains(string(raw), "Car setup") {
		t.Fatal("hidden counter/setup still visible")
	}
	if effect, err := a.event(json.RawMessage(`{"event":{"submit":{"form":"drivers","row":"` + a.expanded + `","values":{"setup":true}}}}`)); err != nil || effect != "none" {
		t.Fatal("hidden setup event accepted")
	}
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.Progress--
	if a.needsView() {
		t.Fatal("hidden timer refreshed view")
	}
	a.hidden[0] |= fieldBit(0, "last") | fieldBit(0, "team")
	a.lastView = a.viewState()
	a.frame.Cars[0].Known |= knownLast
	a.frame.Cars[0].Last = 90000
	a.frame.Cars[0].BadgePNG = testBadgePNG(t)
	if a.needsView() {
		t.Fatal("hidden fields refreshed view")
	}
}

func TestColumnEditorGeneration(t *testing.T) {
	a := App{frame: grid(), data: t.TempDir(), profileName: "main"}
	a.loadColumns()
	clickColumns(t, &a, "columns")
	oldForm := a.columnForm()
	clickColumns(t, &a, "columns-done")
	clickColumns(t, &a, "columns")
	if oldForm == a.columnForm() {
		t.Fatal("selector generation reused")
	}
	handled, effect, err := a.columnEvent("", oldForm, map[string]json.RawMessage{"driver": json.RawMessage("false")})
	if !handled || effect != "none" || err != nil || a.hidden != [3]uint32{} {
		t.Fatal("stale reopened editor changed preferences")
	}
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.Progress++
	a.frame.Cars[0].Last++
	a.frame.Player = 0
	if !a.needsView() {
		t.Fatal("live updates stopped while columns panel was open")
	}
}

func TestInlineColumnsLivePreview(t *testing.T) {
	for _, session := range []uint32{1, 5, 10, 15, 16} {
		a := App{frame: grid(), mode: "full", data: t.TempDir(), profileName: "main"}
		a.loadColumns()
		a.frame.SessionType = session
		layout := layoutFor(a.frame)
		clickColumns(t, &a, "columns")
		blocks := a.view()["body"].(M)["blocks"].([]M)
		if len(blocks) != 2 || blocks[0]["card"] == nil || blocks[1]["table"] == nil {
			t.Fatal("selector did not expand above live table", session)
		}
		panel := blocks[0]["card"].(M)
		if panel["blocks"].([]M)[0]["form"] == nil {
			t.Fatal("selector form missing")
		}
		if _, err := submitColumns(&a, layoutKeys[layout], `{"last":false}`); err != nil {
			t.Fatal(err)
		}
		if !a.columnsOpen {
			t.Fatal("saving collapsed selector")
		}
		if _, exists := rowCells(t, &a, 0)["last"]; exists {
			t.Fatal("preview did not hide column")
		}
		a.frame.Cars[0].Known |= knownBest
		a.frame.Cars[0].Best = 80000
		a.lastView, a.viewSent = a.viewState(), true
		a.frame.Cars[0].Best = 79000
		if !a.needsView() {
			t.Fatal("live table stopped refreshing")
		}
		clickColumns(t, &a, "columns")
		if a.columnsOpen {
			t.Fatal("second click did not collapse selector")
		}
		blocks = a.view()["body"].(M)["blocks"].([]M)
		if len(blocks) != 1 || blocks[0]["table"] == nil {
			t.Fatal("collapsed panel left extra space")
		}
		b := App{frame: a.frame, mode: "full", data: a.data, profileName: "main"}
		b.loadColumns()
		if b.fieldVisible(layout, "last") {
			t.Fatal("preview selection was not saved")
		}
	}
}
func TestInlineColumnsPreserveSetupAndRecoverAllHidden(t *testing.T) {
	a := App{frame: grid(), mode: "full", data: t.TempDir(), profileName: "main"}
	a.loadColumns()
	a.expanded = rowID(a.frame, a.frame.Cars[0])
	clickColumns(t, &a, "columns")
	if a.expanded == "" {
		t.Fatal("opening selector closed setup preview")
	}
	if _, err := submitColumns(&a, "practice", `{"setup":false}`); err != nil {
		t.Fatal(err)
	}
	if a.expanded != "" {
		t.Fatal("hiding setup left detail open")
	}
	for _, col := range columns(layoutPractice) {
		a.hidden[0] |= fieldBit(0, col["key"].(string))
	}
	blocks := a.view()["body"].(M)["blocks"].([]M)
	if len(blocks) != 2 || blocks[0]["card"] == nil || blocks[1]["text"] == nil {
		t.Fatal("all hidden lost inline recovery")
	}
	clickColumns(t, &a, "columns-reset")
	if !a.columnsOpen || len(rowCells(t, &a, 0)) != len(columns(0)) {
		t.Fatal("reset did not restore live preview")
	}
}
