package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func analysisTable(view M, id string) M {
	for _, b := range view["body"].(M)["blocks"].([]M) {
		if v, ok := b["table"].(M); ok && v["id"] == id {
			return v
		}
	}
	return nil
}
func analysisControls(view M) map[string]M {
	out := map[string]M{}
	for _, b := range view["body"].(M)["blocks"].([]M) {
		if v, ok := b["form"].(M); ok && v["id"] == "analysis-controls" {
			for _, f := range v["fields"].([]M) {
				s := f["select"].(M)
				out[s["id"].(string)] = s
			}
		}
	}
	return out
}
func analysisFixture(session int) *App {
	meta := RecordingMeta{ID: strings.Repeat("c", 32), Save: "Career", Session: session, Player: 0}
	m := newModel(meta)
	for i, p := range []int{12, 2, 1, 0} {
		d := &m.Drivers[i]
		d.Name = []string{"Twelfth", "Second", "First", "Unknown"}[i]
		d.Position = p
		d.Grid = i + 1
	}
	m.Drivers[0].Laps[1] = &RecordedLap{Number: 1, Valid: true, Time: 90100, S1: 30000, S2: 30100, S3: 30000}
	m.Drivers[0].Laps[2] = &RecordedLap{Number: 2, Valid: false, Time: 80000}
	m.Drivers[0].Laps[3] = &RecordedLap{Number: 3, Valid: true, Time: 0}
	m.Drivers[0].Laps[4] = &RecordedLap{Number: 4, Valid: true, Time: 90000, S1: 30000, S2: 30000, S3: 30000}
	m.Drivers[1].Laps[1] = &RecordedLap{Number: 1, Valid: true, Time: 91000, S1: 31000, S2: 30000, S3: 30000}
	m.Drivers[1].Laps[2] = &RecordedLap{Number: 2, Valid: false, Time: 79000}
	return &App{rec: &Recorder{items: []RecordingMeta{meta}}, analysis: AnalysisSelection{Model: m, Tab: "stats"}}
}
func TestSessionTabsGridAndPositionOrdering(t *testing.T) {
	for session := 1; session <= 17; session++ {
		a := analysisFixture(session)
		view := a.analysisView()
		var tabs []M
		for _, b := range view["body"].(M)["blocks"].([]M) {
			if v, ok := b["tabs"].(M); ok {
				tabs = v["items"].([]M)
			}
		}
		restricted := session <= 14
		for _, id := range []string{"traces", "tyres", "positions"} {
			found := false
			for _, tab := range tabs {
				found = found || tab["id"] == id
			}
			if found == restricted {
				t.Fatalf("session %d inappropriate %s visibility", session, id)
			}
		}
		if qualifyingSession(session) {
			if len(tabs) != 2 || tabs[0]["id"] != "stats" || tabs[1]["id"] != "advanced" {
				t.Fatal("qualifying navigation", tabs)
			}
			for _, table := range wireNodes(view, "table") {
				for _, col := range table["columns"].([]any) {
					if col.(map[string]any)["title"] == "Grid" {
						t.Fatal("qualifying exposes Grid")
					}
				}
			}
			a.analysis.Tab = "laps"
			a.analysisView()
			if a.analysis.Tab != "stats" {
				t.Fatal("removed qualifying tab remains accessible")
			}
			continue
		}

		results := analysisTable(view, "analysis-results")
		hasGrid := false
		for _, column := range results["columns"].([]M) {
			hasGrid = hasGrid || column["title"] == "Grid"
		}
		if hasGrid == restricted {
			t.Fatalf("session %d inappropriate Grid column", session)
		}
		positions := []string{}
		for _, row := range results["rows"].([][]any) {
			if cell, ok := row[0].(M); ok {
				positions = append(positions, fmt.Sprint(cell["sort"]))
			} else {
				positions = append(positions, row[0].(string))
			}
		}
		if !reflect.DeepEqual(positions, []string{"1", "2", "12", "—"}) {
			t.Fatal("results not numerically sorted", positions)
		}
		if restricted {
			second := "laps"
			if practiceSession(session) {
				second = "practice"
			}
			if len(tabs) != 2 || tabs[0]["id"] != "stats" || tabs[1]["id"] != second {
				t.Fatal("practice/quali must have exactly two tabs", tabs)
			}
			for _, old := range []string{"", "traces", "tyres", "positions"} {
				a.analysis.Tab = old
				view = a.analysisView()
				if a.analysis.Tab != "stats" || analysisTable(view, "analysis-results") == nil {
					t.Fatal("practice/quali did not fall back to results", old)
				}
			}
		}
		if tabs[0]["id"] != "stats" || tabs[0]["label"] != resultTabLabel(session) {
			t.Fatal("first results tab incorrectly labelled", session, tabs)
		}
	}
}
func TestLegacyLapAndSectorComparisonWithoutTraces(t *testing.T) {
	a := analysisFixture(18)
	a.analysis.Tab = "laps"
	view := a.analysisView()
	rows := analysisTable(view, "analysis-laps")["rows"].([][]any)
	if len(rows) != 2 || rows[0][0] != "1" || rows[1][0] != "4" {
		t.Fatal("blank or invalid laps not filtered", rows)
	}
	if analysisTable(view, "analysis-lap-comparison") != nil {
		t.Fatal("comparison displayed by default")
	}
	raw, _ := json.Marshal(M{"event": M{"submit": M{"form": "analysis-controls", "values": M{"driver": "0", "lap": "4", "reference_driver": "1", "reference_lap": "1"}}}})
	if _, e := a.analysisEvent(raw); e != nil {
		t.Fatal(e)
	}
	view = a.analysisView()
	comparison := analysisTable(view, "analysis-lap-comparison")
	if comparison == nil {
		t.Fatal("valid history-only laps cannot be compared")
	}
	rows = comparison["rows"].([][]any)
	if len(rows) != 4 || rows[0][3].(M)["text"] != "-1.000 s" || rows[0][3].(M)["tone"] != "good" || rows[2][3].(M)["text"] != "+0.000 s" {
		t.Fatal("incorrect lap/sector comparison", rows)
	}
	fields := analysisControls(view)
	for _, key := range []string{"lap", "reference_lap"} {
		for _, o := range fields[key]["options"].([]M) {
			if o["value"] == "2" || o["value"] == "3" {
				t.Fatal("invalid/unfinished lap offered for comparison", key, o)
			}
		}
	}
	if lapDelta(30000, 0)["text"] != "—" || lapDelta(60100, 59900)["text"] != "+0.200 s" {
		t.Fatal("unknown or minute-boundary delta incorrect")
	}
	raw, _ = json.Marshal(M{"event": M{"submit": M{"form": "analysis-controls", "values": M{"reference_driver": "none"}}}})
	a.analysisEvent(raw)
	if analysisTable(a.analysisView(), "analysis-lap-comparison") != nil {
		t.Fatal("No comparison failed in laps tab")
	}
}

func TestCompactAnalysisHeaderAndSaveEditor(t *testing.T) {
	a := analysisFixture(8)
	form := func(view M, id string) M {
		for _, b := range view["body"].(M)["blocks"].([]M) {
			if f, ok := b["form"].(M); ok && f["id"] == id {
				return f
			}
		}
		return nil
	}
	view := a.analysisView()
	filters := form(view, "analysis-filters")
	if filters["columns"] != 3 || len(filters["fields"].([]M)) != 3 || form(view, "analysis-save") != nil {
		t.Fatal("single-season header is not compact")
	}
	for _, b := range view["body"].(M)["blocks"].([]M) {
		if b["tabs"] != nil {
			break
		}
		if b["facts"] != nil {
			t.Fatal("diagnostic facts still above tabs")
		}
	}
	click := func() {
		raw, _ := json.Marshal(M{"event": M{"click": M{"id": "analysis-edit-save"}}})
		if _, e := a.analysisEvent(raw); e != nil {
			t.Fatal(e)
		}
	}
	click()
	view = a.analysisView()
	editor := form(view, "analysis-save")
	if editor == nil {
		t.Fatal("save editor cannot be opened")
	}
	if editor["fields"].([]M)[0]["text"].(M)["help"] != nil {
		t.Fatal("long help text remains in header")
	}
	click()
	if form(a.analysisView(), "analysis-save") != nil {
		t.Fatal("save editor cannot be closed")
	}
	click()
	raw, _ := json.Marshal(M{"event": M{"submit": M{"form": "analysis-filters", "values": M{"season": "43"}}}})
	a.analysisEvent(raw)
	if a.analysis.EditSave {
		t.Fatal("editing remained open after session filters changed")
	}
	a.analysis.Season = "unknown"
	other := a.rec.items[0]
	other.ID = strings.Repeat("d", 32)
	other.Season = 43
	a.rec.items = append(a.rec.items, other)
	filters = form(a.analysisView(), "analysis-filters")
	if len(filters["fields"].([]M)) != 4 {
		t.Fatal("multiple seasons lost their selector")
	}
}
