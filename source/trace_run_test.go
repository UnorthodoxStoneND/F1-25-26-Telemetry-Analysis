package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestQualifyingReusesLapNumberAcrossRuns(t *testing.T) {
	d := RecordedDriver{Laps: map[int]*RecordedLap{6: {Number: 6, Time: 79561, Valid: true}}}
	d.Samples = []TraceSample{
		{Lap: 6, Status: 1, Time: 540, Distance: 2, LapMS: 16, Speed: 310},
		{Lap: 6, Status: 1, Time: 541, Distance: 84, LapMS: 983, Speed: 320},
		{Lap: 6, Status: 0, Time: 542, Distance: 5243, LapMS: 999, Speed: 0},
		{Lap: 6, Status: 3, Time: 933, Distance: 1, LapMS: 0, Speed: 80},
		{Lap: 6, Status: 3, Time: 1036, Distance: 5275, LapMS: 0, Speed: 300},
		{Lap: 6, Status: 1, Time: 1036.5, Distance: 3, LapMS: 33, Speed: 310},
		{Lap: 6, Status: 1, Time: 1116, Distance: 5274, LapMS: 79522, Speed: 315},
	}
	got := d.trace(6)
	if len(got) != 2 || got[0].Time != 1036.5 {
		t.Fatalf("mixed qualifying attempts: %+v", got)
	}
	if warning := traceCoverage(got, 79561); warning != "" {
		t.Fatal(warning)
	}
	if warning := traceCoverage(got[:1], 79561); warning == "" {
		t.Fatal("partial completion not labelled")
	}
}
func TestTimedRunResetsAndRacePitSamples(t *testing.T) {
	d := RecordedDriver{Samples: []TraceSample{{Lap: 1, Status: 4, Distance: 500, Time: 1, LapMS: 10000}, {Lap: 1, Status: 4, Distance: 1000, Time: 2, LapMS: 20000}, {Lap: 1, Status: 4, Distance: 3, Time: 3, LapMS: 0}, {Lap: 1, Status: 4, Distance: 500, Time: 4, LapMS: 10000}}}
	got := d.trace(1)
	if len(got) != 2 || got[0].Time != 3 {
		t.Fatal("distance/timer reset did not separate attempts", got)
	}
}
func TestCompletedSnapshotFooterAndLateHistory(t *testing.T) {
	r, e := openRecorder(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer r.finish("interrupted")
	r.ingest(lapAt(2026, 1, 0, 1, 0), "p")
	r.ingest(eventAt(2026, 2, 1, "SEND"), "p")
	check := func() {
		t.Helper()
		// Analysis explicitly flushes before reading an active recording.
		if e := r.flush(); e != nil {
			t.Fatal(e)
		}
		partial, e := readJournal(filepath.Join(r.dir, r.current.ID+".udp.gz"), func([]byte) {})
		if e != nil || partial {
			t.Fatal("completed snapshot incorrectly appears truncated", partial, e)
		}
	}
	check()
	r.ingest(analysisPacket(2026, 11, 3, 2), "p")
	check()
	if r.current.Packets != 3 {
		t.Fatal("late history missing")
	}
}

// Optional read-only replay of a local real recording. Output stays in the
// workspace and no installed telemetry data is modified by this test.
func TestReplayTraceFromEnvironment(t *testing.T) {
	path := os.Getenv("UDP_TELEMETRY_REPLAY")
	if path == "" {
		t.Skip("no replay fixture configured")
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
	if out := os.Getenv("UDP_TELEMETRY_SUMMARY"); out != "" {
		if e := renderSessionSummary(out, sessionSummary(m)); e != nil {
			t.Fatal(e)
		}
		t.Log("Session summary:", sessionSummary(m))
	}
	selected, comparison := m.Drivers[23].trace(6), m.Drivers[22].trace(3)
	if len(selected) < 1000 || selected[0].Time < 1000 || len(comparison) < 1000 || comparison[0].Time < 1000 {
		t.Fatalf("old qualifying attempts included: selected %d comparison %d", len(selected), len(comparison))
	}
	for _, samples := range [][]TraceSample{selected, comparison} {
		for _, s := range samples {
			if s.Status != 1 {
				t.Fatal("garage/out-lap contaminates comparison")
			}
		}
	}
	t.Logf("selected: %d samples, %.3f–%.3fs; comparison: %d samples, %.3f–%.3fs; %s", len(selected), selected[0].Time, selected[len(selected)-1].Time, len(comparison), comparison[0].Time, comparison[len(comparison)-1].Time, traceCoverage(comparison, m.Drivers[22].Laps[3].Time))
	out := os.Getenv("UDP_TELEMETRY_CHART")
	if out != "" {
		if e = renderTrace(out, selected, comparison, 1); e != nil {
			t.Fatal(e)
		}
	}
	if single := os.Getenv("UDP_TELEMETRY_SINGLE_CHART"); single != "" {
		if e = renderTrace(single, selected, nil, 0); e != nil {
			t.Fatal(e)
		}
	}
	if out := os.Getenv("UDP_TELEMETRY_SPARSE_CHART"); out != "" {
		goethe := m.Drivers[22].trace(2)
		if len(goethe) < 10 {
			t.Fatal("GOETHE lap 2 missing")
		}
		t.Logf("GOETHE lap 2: %d samples, frame step %d, %.3f–%.3f s; %s", len(goethe), traceFrameStep(goethe), goethe[0].Time, goethe[len(goethe)-1].Time, traceResolution(goethe))
		if e = renderTrace(out, m.Drivers[23].trace(5), goethe, 0); e != nil {
			t.Fatal(e)
		}
	}
}
