package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPracticeAnalysisDataAndNavigation(t *testing.T) {
	a := analysisFixture(1)
	a.data = t.TempDir()

	m := a.analysis.Model
	m.Drivers[0].TeamKnown = true
	m.Drivers[0].TeamID = 7
	m.Drivers[1].TeamKnown = true
	m.Drivers[1].TeamID = 7
	m.Drivers[1].Laps[1] = &RecordedLap{Time: 89000, S1: 29000, S2: 31000, S3: 29000, Valid: true}
	m.Drivers[1].Laps[2] = &RecordedLap{Time: 60000, S1: 10000, S2: 10000, S3: 10000, Valid: false}
	leaders := teamSectorLeaders(m)
	if leaders[0][0].Time != 29000 || leaders[1][0].Time != 30000 || leaders[2][0].Time != 29000 {
		t.Fatal("team sector minima should exclude invalid laps", leaders)
	}
	if leaders[0][0].Driver != leaders[1][0].Driver {
		t.Fatal("team color changed by sector")
	}
	mean, sd := practiceConsistency([]float64{80, 90, 100})
	if mean != 90 || math.Abs(sd-math.Sqrt(200./3)) > .000001 {
		t.Fatal(mean, sd)
	}
	best, average, count := practiceTimes(&m.Drivers[1])
	if best != 89000 || average != 89000 || count != 2 {
		t.Fatal("practice fastest/mean/lap count", best, average, count)
	}
	view := a.analysisView()
	titles := []string{}
	for _, col := range analysisTable(view, "analysis-results")["columns"].([]M) {
		titles = append(titles, col["title"].(string))
	}
	if !reflect.DeepEqual(titles, []string{"POS", "Full Driver Name", "Team", "Compounds", "Laps", "Fastest Lap", "Gap", "Avg Time"}) {
		t.Fatal(titles)
	}
	a.analysis.Tab = "practice"
	view = a.analysisView()
	if len(wireNodes(view, "chart")) != 5 || strings.Contains(string(mustJSON(view)), "Laps & sectors") {
		t.Fatal("practice charts/navigation")
	}
	a.raceSelectionEvent("analysis-race-all", "", nil, nil)
	if len(a.selectedRaceDrivers(m)) != 4 {
		t.Fatal("practice chips do not select all drivers")
	}
	a.analysis.Tab = "laps"
	a.analysisView()
	if a.analysis.Tab != "stats" {
		t.Fatal("removed practice tab remains accessible")
	}
}
func TestPracticePreviewFromEnvironment(t *testing.T) {
	path, out := os.Getenv("UDP_TELEMETRY_PRACTICE_META"), os.Getenv("UDP_TELEMETRY_PRACTICE_PREVIEW")
	if path == "" || out == "" {
		t.Skip("no practice preview configured")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var meta RecordingMeta
	if e = json.Unmarshal(b, &meta); e != nil {
		t.Fatal(e)
	}
	m, e := loadAnalysis(filepath.Dir(path), meta)
	if e != nil {
		t.Fatal(e)
	}
	a := App{data: out, rec: &Recorder{items: []RecordingMeta{meta}}, analysis: AnalysisSelection{Model: m}}
	a.ensureRaceSelection(m)
	for _, i := range raceDriverIndices(m) {
		a.analysis.RaceDrivers |= 1 << uint(i)
	}
	for _, tab := range []string{"stats", "practice"} {
		a.analysis.Tab = tab
		view := a.analysisView()
		b, _ = json.MarshalIndent(view, "", "  ")
		if e = os.MkdirAll(out, 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(out, tab+".json"), b, 0644); e != nil {
			t.Fatal(e)
		}
		t.Log(tab, len(wireNodes(view, "image")), "images")
	}
}
