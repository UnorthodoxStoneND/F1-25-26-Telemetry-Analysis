package main

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionSelectionOpensResultsFirst(t *testing.T) {
	a := analysisFixture(15)
	a.analysis.Save = "Career"
	a.analysis.Tab = "traces"
	raw, _ := json.Marshal(M{"event": M{"submit": M{"form": "analysis-filters", "values": M{"save": "Other career"}}}})
	if _, e := a.analysisEvent(raw); e != nil {
		t.Fatal(e)
	}
	if a.analysis.Tab != "stats" {
		t.Fatal("new session retained old analysis tab")
	}
}

func TestSessionSummaryDecodingAndRewind(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		m := newModel(RecordingMeta{Format: format, Session: 15, Track: -1})
		b := analysisPacket(format, 1, 1, 1)
		b[29], b[30], b[31], b[32], b[35], b[36] = 3, 45, 24, 34, 15, 16
		m.ingest(b)
		values := sessionSummary(m)
		want := []string{"Brazil", "Interlagos", "34", "24°C", "45°C", "Light rain"}
		for i, value := range values {
			if value["value"] != want[i] {
				t.Fatalf("format %d summary %d: %v", format, i, value)
			}
		}
		putFloat(b[15:], 2)
		b[29], b[30], b[31] = 0, 0, 251
		m.ingest(b)
		values = sessionSummary(m)
		if values[3]["value"] != "-5°C" || values[4]["value"] != "0°C" || values[5]["value"] != "Clear" {
			t.Fatal("signed/zero values or latest weather lost", values)
		}
		m.rewind(1.5)
		if sessionSummary(m)[3]["value"] != "24°C" {
			t.Fatal("flashback did not restore conditions")
		}
		m.Meta.Session = 8
		if sessionSummary(m)[2]["value"] != "Timed session" {
			t.Fatal("qualifying given invented lap count")
		}
	}
}
func TestUnknownSummaryAndTrackIDs(t *testing.T) {
	m := newModel(RecordingMeta{Track: -1, Session: 15})
	for _, v := range sessionSummary(m) {
		if v["value"] != "—" {
			t.Fatal("missing measurements turned into real values", v)
		}
	}
	if circuits[39].Country != "United Kingdom" || circuits[40].Country != "Austria" || circuits[41].Country != "Netherlands" || circuits[42].Circuit != "MADRING" {
		t.Fatal("reverse/Madrid lookup")
	}
	if resultTabLabel(5) != "Qualifying Results" || resultTabLabel(14) != "Qualifying Results" || resultTabLabel(1) != "Overview" || resultTabLabel(15) != "Overview" {
		t.Fatal("contextual results labels")
	}
}
func TestSummaryStripRendering(t *testing.T) {
	m := newModel(RecordingMeta{Session: 15, Track: 16})
	m.Conditions = &SessionConditions{Track: 16, Air: 24, Surface: 45, Weather: 3, TotalLaps: 34}
	path := filepath.Join(t.TempDir(), "summary.png")
	if e := renderSessionSummary(path, sessionSummary(m)); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	im, e := png.Decode(f)
	if e != nil {
		t.Fatal(e)
	}
	if im.Bounds().Dx() != 3600 || im.Bounds().Dy() != 144 {
		t.Fatal("summary is not a single compact row")
	}
	white := 0
	for y := 60; y < 125; y++ {
		for x := 100; x < 590; x++ {
			r, g, b, _ := im.At(x, y).RGBA()
			if r > 50000 && g > 50000 && b > 50000 {
				white++
			}
		}
	}
	if white < 100 {
		t.Fatal("summary text was not rendered")
	}
}
