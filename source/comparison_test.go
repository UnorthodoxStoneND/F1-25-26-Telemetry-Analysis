package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNoComparisonControlsAndSingleTrace(t *testing.T) {
	meta := RecordingMeta{ID: strings.Repeat("b", 32), Save: "Test", Player: 0}
	m := newModel(meta)
	for i := 0; i < 2; i++ {
		d := &m.Drivers[i]
		d.Name = []string{"Selected", "Reference"}[i]
		d.Laps[1] = &RecordedLap{Number: 1, Time: 10000, Valid: true}
		d.Samples = []TraceSample{{Lap: 1, Status: 1, Time: 1, Distance: 0, LapMS: 0, Speed: 200}, {Lap: 1, Status: 1, Time: 1.1, Distance: 100, LapMS: 100, Speed: 210}}
	}
	a := &App{data: t.TempDir(), rec: &Recorder{items: []RecordingMeta{meta}}, analysis: AnalysisSelection{Model: m, Tab: "traces", Driver: 0, Lap: 1}}
	controls := func(view M) map[string]M {
		t.Helper()
		out := map[string]M{}
		for _, block := range view["body"].(M)["blocks"].([]M) {
			if f, ok := block["form"].(M); ok && f["id"] == "analysis-controls" {
				for _, field := range f["fields"].([]M) {
					v := field["select"].(M)
					out[v["id"].(string)] = v
				}
			}
		}
		return out
	}
	hasOrange := func(view M) bool {
		t.Helper()
		charts := wireNodes(view, "chart")
		if len(charts) != 5 {
			t.Fatal("five native traces missing")
		}
		for _, c := range charts {
			for _, v := range c["series"].([]any) {
				if v.(map[string]any)["color"] == "#FFA752" {
					return true
				}
			}
		}
		return false
	}
	event := func(values M, changed []string) {
		t.Helper()
		raw, _ := json.Marshal(M{"event": M{"submit": M{"form": "analysis-controls", "values": values, "changed": changed}}})
		if handled, e := a.analysisEvent(raw); !handled || e != nil {
			t.Fatal("event", handled, e)
		}
	}
	view := a.analysisView()
	fields := controls(view)
	if fields["reference_driver"]["value"] != "none" || fields["reference_lap"] != nil || hasOrange(view) {
		t.Fatal("default no-comparison mode is ambiguous or draws a reference")
	}
	if !strings.Contains(view["subtitle"].(string), addonVersion) {
		t.Fatal("exact running version missing")
	}
	event(M{"reference_driver": "1"}, []string{"reference_driver"})
	view = a.analysisView()
	if controls(view)["reference_lap"] == nil || !hasOrange(view) {
		t.Fatal("comparison was not enabled")
	}
	// Native forms submit all values; only Changed fields may affect mode.
	event(M{"reference_driver": "none", "reference_lap": "1", "lap": "1"}, []string{"reference_driver"})
	view = a.analysisView()
	if a.analysis.Compare || controls(view)["reference_driver"]["value"] != "none" || controls(view)["reference_lap"] != nil || hasOrange(view) {
		t.Fatal("stale form values restored a disabled comparison")
	}
	event(M{"lap": "1", "reference_driver": "1", "reference_lap": "1"}, []string{"lap"})
	if a.analysis.Compare {
		t.Fatal("unchanged stale comparison values were applied")
	}
	event(M{"reference_driver": "1", "reference_lap": "1"}, nil)
	a.analysisView()
	event(M{"reference_lap": "0"}, []string{"reference_lap"})
	view = a.analysisView()
	if a.analysis.Compare || hasOrange(view) {
		t.Fatal("No comparison lap did not disable comparison")
	}
}
