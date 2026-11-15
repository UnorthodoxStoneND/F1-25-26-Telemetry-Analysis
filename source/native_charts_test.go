package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeSmallSampleWhiskersEncloseQuartiles(t *testing.T) {
	for _, times := range [][]uint32{{90000}, {90000, 91000}, {90000, 90000, 90000, 200000}, {10000, 90000, 90000, 90000}} {
		m := newModel(RecordingMeta{})
		m.Drivers[0].Name = "Driver"
		for n, v := range times {
			m.Drivers[0].Laps[n+1] = &RecordedLap{Time: v, Valid: true}
		}
		c := nativeDistribution(m, []int{0}, false, false)["chart"].(M)
		b := c["series"].([]M)[0]["values"].([]M)[0]
		if b["min"].(float64) > b["q1"].(float64) || b["q3"].(float64) > b["max"].(float64) {
			t.Fatal("whisker outside box ordering", b)
		}
	}
}

func TestNativeSavedRecordingReplay(t *testing.T) {
	dir, out := os.Getenv("UDP_TELEMETRY_NATIVE_REPLAY"), os.Getenv("UDP_TELEMETRY_NATIVE_REPLAY_VIEWS")
	if dir == "" || out == "" {
		t.Skip("optional read-only replay of saved sessions")
	}
	files, e := filepath.Glob(filepath.Join(dir, "*.json"))
	if e != nil {
		t.Fatal(e)
	}
	metas := []RecordingMeta{}
	for _, path := range files {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var meta RecordingMeta
		if json.Unmarshal(b, &meta) == nil && meta.State == "completed" && meta.Session > 0 && meta.Session <= 17 {
			metas = append(metas, meta)
		}
	}
	f, e := os.Create(out)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	for _, meta := range metas {
		paths := []string{filepath.Join(dir, meta.ID+".json"), filepath.Join(dir, meta.ID+".udp.gz")}
		before := map[string][32]byte{}
		for _, p := range paths {
			b, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			before[p] = sha256.Sum256(b)
		}
		m, e := loadAnalysis(dir, meta)
		if e != nil {
			t.Fatal(e)
		}
		a := App{data: t.TempDir(), rec: &Recorder{items: []RecordingMeta{meta}}, analysis: AnalysisSelection{Model: m}}
		tabs := []string{"stats", "traces", "tyres", "positions", "progression"}
		if practiceSession(meta.Session) {
			tabs = []string{"stats", "practice"}
		}
		if qualifyingSession(meta.Session) {
			tabs = []string{"stats", "advanced"}
			a.analysis.Qualifying = loadQualifyingWeekend(dir, meta, metas, m)
		}
		a.ensureRaceSelection(m)
		for _, i := range raceDriverIndices(m) {
			a.analysis.RaceDrivers |= 1 << uint(i)
		}
		for _, tab := range tabs {
			a.analysis.Tab = tab
			start := time.Now()
			view := a.analysisView()
			if len(wireNodes(view, "image")) != 0 {
				t.Fatal("replay generated a chart image")
			}
			if e := json.NewEncoder(f).Encode(view); e != nil {
				t.Fatal(e)
			}
			t.Logf("session %d %s: %d charts in %s", meta.Session, tab, len(wireNodes(view, "chart")), time.Since(start).Round(time.Millisecond))
		}
		for _, p := range paths {
			b, e := os.ReadFile(p)
			if e != nil || sha256.Sum256(b) != before[p] {
				t.Fatal("source recording changed", p, e)
			}
		}
	}
	t.Logf("Read-only replayed %d completed sessions; original metadata/journal hashes unchanged", len(metas))
}

func TestNativeSummaryNeedsNoWritableCache(t *testing.T) {
	a := App{data: t.TempDir()}
	m := newModel(RecordingMeta{Session: 15, Track: 16})
	for n := 0; n < 10; n++ {
		m.Conditions = &SessionConditions{Air: n, Surface: 45, Track: 16, TotalLaps: 34}
		block := a.summaryBlock(m)
		if len(block["stats"].([]M)) != 6 || block["image"] != nil {
			t.Fatal(block)
		}
	}
	entries, _ := os.ReadDir(a.data)
	if len(entries) != 0 {
		t.Fatal("summary wrote files", entries)
	}
}
func TestNativeChartsGapsStylesAndThinning(t *testing.T) {
	c := nativeChart("test", "test", "", nativeAxis("Lap", "integer"), nativeAxis("Value", "number"), 300)
	nativeRaceLine(c, "driver", "#57B7FF", []racePoint{{X: 1, Y: 90}, {X: 2, Y: 91, Dashed: true}, {X: 3, Y: 92}, {X: 5, Y: 89}, {X: 6, Y: 90}}, true, false, true)
	if len(c["series"].([]M)) != 2 {
		t.Fatal("pit segments lost")
	}
	encoded, _ := json.Marshal(c)
	var normalized map[string]any
	_ = json.Unmarshal(encoded, &normalized)
	sawDash, sawBreak := false, false
	for _, s := range normalized["series"].([]any) {
		data := s.(map[string]any)
		if data["dashed"] == true {
			sawDash = true
		}
		for _, p := range data["points"].([]any) {
			if p == nil {
				sawBreak = true
			}
		}
	}
	if !sawDash || !sawBreak {
		t.Fatal("gap or dashed edge missing", string(encoded))
	}
	points := make([]racePoint, 15000)
	for i := range points {
		points[i] = racePoint{X: float64(i), Y: float64(i % 2)}
	}
	points[8123].Y = 9999
	reduced := nativeThinTrace(points)
	peak := false
	for _, p := range reduced {
		peak = peak || p.Y == 9999
	}
	if len(reduced) > 2000 || !peak {
		t.Fatal("thinning lost extrema or limit", len(reduced), peak)
	}
}
func TestNativeViewsDoNotWriteChartFiles(t *testing.T) {
	out := os.Getenv("UDP_TELEMETRY_NATIVE_VIEWS")
	var export *os.File
	if out != "" {
		var e error
		export, e = os.Create(out)
		if e != nil {
			t.Fatal(e)
		}
		defer export.Close()
	}
	emit := func(v M) {
		t.Helper()
		if len(wireNodes(v, "image")) != 0 {
			t.Fatal("analysis emitted a bitmap")
		}
		if export != nil {
			if e := json.NewEncoder(export).Encode(v); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, kind := range []int{1, 2, 3, 4, 8, 9, 13, 14, 15, 16, 17} {
		a := analysisFixture(kind)
		a.data = t.TempDir()

		tabs := []string{"stats", "traces", "tyres", "positions", "progression"}
		if practiceSession(kind) {
			tabs = []string{"stats", "practice"}
		}
		if qualifyingSession(kind) {
			tabs = []string{"stats", "advanced"}
		}
		for pass := 0; pass < 2; pass++ {
			for _, tab := range tabs {
				a.analysis.Tab = tab
				if raceSession(kind) || practiceSession(kind) {
					a.ensureRaceSelection(a.analysis.Model)
					if pass == 0 {
						a.analysis.RaceDrivers = 15
					} else {
						a.analysis.RaceDrivers = 1
					}
				}
				emit(a.analysisView())
			}
		}
		entries, _ := os.ReadDir(a.data)
		if len(entries) != 0 {
			t.Fatal("chart files accumulated", kind, entries)
		}
	}
	for _, kinds := range [][]int{{5, 6, 7}, {10, 11, 12}} {
		q := qualifyingFixture(kinds)
		m := q.Stages[2].Model
		a := App{data: t.TempDir(), rec: &Recorder{items: []RecordingMeta{m.Meta}}, analysis: AnalysisSelection{Model: m, Qualifying: q}}
		for _, tab := range []string{"stats", "advanced"} {
			a.analysis.Tab = tab
			emit(a.analysisView())
		}
	}
	// Exercise full-grid trace payload limits and sparse-frame breaks.
	a := analysisFixture(15)
	m := a.analysis.Model
	for i := 0; i < 24; i++ {
		m.Drivers[i] = m.Drivers[0]
		m.Drivers[i].Name = fmt.Sprintf("Driver %d", i)
		m.Drivers[i].Samples = make([]TraceSample, 12000)
		for n := range m.Drivers[i].Samples {
			m.Drivers[i].Samples[n] = TraceSample{Lap: 1, Status: 1, Frame: n + 1, Time: float64(n) / 100, Distance: float64(n), Speed: 200 + 100*math.Sin(float64(n)/100), Gear: 6}
		}
	}
	a.ensureRaceSelection(m)
	a.analysis.RaceDrivers = (1 << 24) - 1
	a.analysis.Tab = "traces"
	emit(a.analysisView())
}
func TestOldLaunchersNeverReceiveNativeChartBlocks(t *testing.T) {
	for _, capabilities := range [][]string{nil, {}, {"chart"}, {"view.chart.future"}} {
		if nativeChartsSupported(capabilities) {
			t.Fatal("unsupported capability accepted", capabilities)
		}
	}
	if !nativeChartsSupported([]string{"other", "view.chart"}) {
		t.Fatal("released chart capability rejected")
	}
	a := analysisFixture(15)
	a.launcherKnown = true
	a.nativeCharts = false
	view := a.analysisView()
	if len(wireNodes(view, "chart")) != 0 {
		t.Fatal("old launcher received unknown chart variant")
	}
	if export := os.Getenv("UDP_TELEMETRY_NATIVE_VIEWS"); export != "" {
		f, e := os.OpenFile(export, os.O_APPEND|os.O_WRONLY, 0644)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		_ = json.NewEncoder(f).Encode(view)
	}
}
