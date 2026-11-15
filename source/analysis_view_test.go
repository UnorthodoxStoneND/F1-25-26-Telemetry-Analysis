package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAnalysisStatsRelevantToSessionAndNativeCells(t *testing.T) {
	for _, session := range []int{1, 5, 10, 15, 16} {
		meta := RecordingMeta{ID: strings.Repeat("a", 32), Save: "Career", Session: session, Track: 7, Started: time.Now()}
		m := newModel(meta)
		m.Drivers[0].Name = "Driver"
		m.Drivers[0].lapRecord(1).Position = 2
		a := &App{data: t.TempDir(), rec: &Recorder{items: []RecordingMeta{meta}}, analysis: AnalysisSelection{Model: m, Tab: "stats"}}
		b, e := json.Marshal(a.analysisView())
		if e != nil {
			t.Fatal(e)
		}
		has := strings.Contains(string(b), "Overtakes recorded")
		if has != (session >= 15) {
			t.Fatalf("race-only stats in session %d: %s", session, b)
		}
		a.analysis.Tab = "positions"
		view := a.analysisView()
		body := view["body"].(M)
		for _, block := range body["blocks"].([]M) {
			if table, ok := block["table"].(M); ok {
				for _, row := range table["rows"].([][]any) {
					for _, cell := range row {
						if _, ok := cell.(string); !ok {
							t.Fatalf("native cell must be string/spec, got %T", cell)
						}
					}
				}
			}
		}
	}
}
func TestPlayerPrivacyAndMissingLapPacketBound(t *testing.T) {
	m := newModel(RecordingMeta{Format: 2026, Player: 0})
	p := analysisPacket(2026, 4, 1, 0)
	copy(p[40:], "Player")
	copy(p[100:], "Private opponent")
	m.ingest(p)
	if m.Drivers[0].Restricted || !m.Drivers[1].Restricted {
		t.Fatal("privacy")
	}
	for i := 1; i < 300; i++ {
		m.ingest(traceAt(2026, uint32(i), float32(i), 100))
	}
	if len(m.pending) > 65 {
		t.Fatal("unpaired samples are unbounded", len(m.pending))
	}
}
func TestUIDZeroEndAndExplicitSameUIDRestart(t *testing.T) {
	r, e := openRecorder(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	r.ingest(lapAt(2026, 1, 0, 1, 0), "p")
	end := eventAt(2026, 0, 0, "SEND")
	for i := 7; i < 15; i++ {
		end[i] = 0
	}
	r.ingest(end, "p")
	if r.current == nil || r.current.State != "completed" {
		t.Fatal("UID 0 end")
	}
	first := r.current.ID
	before := r.current.Packets
	r.ingest(lapAt(2026, 3, 1, 1, 1), "p")
	if r.current == nil || r.current.State != "completed" || r.current.Packets != before+1 {
		t.Fatal("late data was not retained in completed session")
	}
	r.ingest(eventAt(2026, 4, 0, "SSTA"), "p")
	if r.current == nil || r.current.ID == first {
		t.Fatal("explicit restart rejected")
	}
	r.finish("interrupted")
}
