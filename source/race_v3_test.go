package main

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func wireNodes(value any, key string) []map[string]any {
	b, _ := json.Marshal(value)
	var root any
	_ = json.Unmarshal(b, &root)
	out := []map[string]any{}
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if child, ok := v[key].(map[string]any); ok {
				out = append(out, child)
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(root)
	return out
}
func TestRaceV3TabsSharedSelectionAndOverview(t *testing.T) {
	a := analysisFixture(15)
	a.data = t.TempDir()
	a.displayCache = filepath.Join(a.data, "launcher-cache")
	a.analysis.Tab = "traces"
	view := a.analysisView()
	tabs := wireNodes(view, "tabs")[0]["items"].([]any)
	labels := []string{}
	for _, v := range tabs {
		labels = append(labels, v.(map[string]any)["label"].(string))
	}
	if !reflect.DeepEqual(labels, []string{"Overview", "Lap Analysis", "Pit Stops", "Race Strategy", "Race Progression"}) {
		t.Fatal(labels)
	}
	if len(a.selectedRaceDrivers(a.analysis.Model)) != 1 || a.analysis.RaceDrivers != 1 {
		t.Fatal("default was not the player")
	}
	images := wireNodes(view, "image")
	if len(images) != 7 {
		t.Fatal("expected summary, legend and five separate graphics", len(images))
	}
	grids := wireNodes(view, "grid")
	if len(grids) != 1 || !reflect.DeepEqual(grids[0]["columns"], []any{float64(3), float64(2)}) {
		t.Fatal("lap times are not alongside the charts", grids)
	}
	a.raceSelectionEvent("analysis-race-all", "", nil, nil)
	if a.analysis.RaceDrivers != 15 {
		t.Fatal("all drivers selection", a.analysis.RaceDrivers)
	}
	raw, _ := json.Marshal(M{"event": M{"click": M{"id": "analysis-race-driver-" + a.analysis.Model.Meta.ID + "-1"}}})
	if _, e := a.analysisEvent(raw); e != nil {
		t.Fatal(e)
	}
	if a.analysis.RaceDrivers != 13 {
		t.Fatal("chip click did not toggle only its driver", a.analysis.RaceDrivers)
	}
	for _, tab := range []string{"tyres", "positions", "progression", "traces"} {
		a.analysis.Tab = tab
		view = a.analysisView()
		if a.analysis.RaceDrivers != 13 {
			t.Fatal("tab reset selection", tab)
		}
		for _, im := range wireNodes(view, "image") {
			path := im["path"].(string)
			if !strings.HasPrefix(path, a.displayCache+string(os.PathSeparator)) {
				t.Fatal("launcher rejects image", path)
			}
			if _, e := os.Stat(path); e != nil {
				t.Fatal(e)
			}
		}
	}
	a.raceSelectionEvent("analysis-race-none", "", nil, nil)
	a.analysis.Tab = "stats"
	view = a.analysisView()
	if len(analysisTable(view, "analysis-results")["rows"].([][]any)) != 4 {
		t.Fatal("Overview was filtered by shared selection")
	}
	if strings.Contains(string(mustJSON(view)), "analysis-race-driver-") {
		t.Fatal("Overview exposes race filtering controls")
	}
	a.analysis.Tab = "positions"
	if !strings.Contains(string(mustJSON(a.analysisView())), "No drivers selected") {
		t.Fatal("clear selection did not show empty state")
	}
	meta := a.analysis.Model.Meta
	meta.ID = strings.Repeat("d", 32)
	meta.Player = 1
	next := newModel(meta)
	next.Drivers = a.analysis.Model.Drivers
	a.ensureRaceSelection(next)
	if a.analysis.RaceDrivers != 2 || a.analysis.RaceLap != 1 {
		t.Fatal("new session did not reset to its player")
	}
	for _, session := range []int{1, 5, 8, 10, 14} {
		other := analysisFixture(session)
		v := other.analysisView()
		items := wireNodes(v, "tabs")[0]["items"].([]any)
		if len(items) != 2 || strings.Contains(string(mustJSON(v)), "analysis-race-") {
			t.Fatal("practice/qualifying page changed", session)
		}
	}
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func TestRaceMetricsMedianFlagsGapsAndPitMean(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		m := newModel(RecordingMeta{Format: format, Session: 15})
		lights := eventAt(format, 1, 0, "STLG")
		lights[33] = 1
		m.ingest(lights)
		session := analysisPacket(format, 1, 1, 0)
		session[35], session[153] = 15, 1
		m.ingest(session)
		lap := lapAt(format, 2, 1, 1, 100)
		r := lap[29:]
		r[32], r[34], r[37], r[44] = 2, 1, 1, 4
		binary.LittleEndian.PutUint16(r[17:], 3456)
		r[19] = 1
		m.ingest(lap)
		for i, time := range []uint32{90000, 100000, 200000} {
			m.Drivers[i].Laps[1] = &RecordedLap{Number: 1, Time: time, Valid: i != 2, S1: 30000, S2: 30000, S3: 30000}
		}
		m.Drivers[3].Laps[1] = &RecordedLap{Number: 1, Time: 0, Valid: true}
		metrics := m.metrics()
		if metrics.Median[1] != 100 {
			t.Fatal("median excludes invalid lap or includes missing time", metrics.Median)
		}
		got := metrics.Laps[0][1]
		if got.GapMS != 63456 || got.Position != 2 || got.Flags&(flagPitIn|flagSC|flagGapKnown) != (flagPitIn|flagSC|flagGapKnown) {
			t.Fatal("UDP gap/flags decoding", got)
		}
		if metrics.Laps[2][1].Flags&flagInvalid == 0 {
			t.Fatal("invalid completed lap not marked")
		}
		lap = lapAt(format, 3, 2, 1, 150)
		lap[61], lap[73] = 2, 4
		m.ingest(lap)
		if m.metrics().Laps[0][1].GapMS != 63456 || m.metrics().Laps[0][1].Flags&flagGapKnown == 0 {
			t.Fatal("brief unknown sample erased the last valid gap on this lap")
		}
	}
	d := &RecordedDriver{Completed: 20, Classified: true, Stints: []RecordedStint{{9, 17}, {255, 18}}, Pits: []PitVisit{{Complete: true, LaneMS: 17000, StopMS: 2500}, {Complete: true, LaneMS: 5000, StopMS: 0}, {Complete: false, LaneMS: 22000, StopMS: 3000}, {Complete: true, LaneMS: 23000, StopMS: 2700}}}
	avg, n := averagePitLane(d)
	if avg != 20 || n != 2 {
		t.Fatal("mean should use complete lane visits, exclude drive-through/incomplete", avg, n)
	}
	m := newModel(RecordingMeta{})
	m.Drivers[0] = *d
	stints := m.raceStints(0)
	if len(stints) != 2 || stints[0].End-stints[0].Start+1 != 9 || stints[1].End-stints[1].Start+1 != 11 {
		t.Fatal("stacked columns do not total completed distance", stints)
	}
	if math.Abs(quantile([]float64{1, 2, 3, 4}, .25)-1.75) > .0001 || quantile([]float64{1, 2, 3, 4}, .5) != 2.5 {
		t.Fatal("boxplot quantiles")
	}
}
func TestRaceObservationFlashbackRestoresFlags(t *testing.T) {
	d := RecordedDriver{}
	d.observeRace(RaceObservation{Time: 1, Frame: 10, Lap: 1, Position: 3, GapMS: 5000, Flags: flagGapKnown})
	d.observeRace(RaceObservation{Time: 2, Frame: 20, Lap: 1, Position: 2, GapMS: 6000, Flags: flagGapKnown | flagSC | flagPitIn})
	d.observeRace(RaceObservation{Time: 3, Frame: 30, Lap: 2, Position: 2, GapMS: 7000, Flags: flagGapKnown | flagPitOut})
	d.rewindRaceObservations(1.5, nil)
	m := newModel(RecordingMeta{})
	m.Drivers[0] = d
	m.Drivers[0].Laps = map[int]*RecordedLap{1: {Time: 90000, Valid: true}}
	got := m.metrics().Laps[0][1]
	if got.Position != 3 || got.GapMS != 5000 || got.Flags&(flagSC|flagPitIn|flagPitOut) != 0 {
		t.Fatal("future context survived flashback", got)
	}
}
func TestRaceV3PitTraceAndClassifiedFinishGaps(t *testing.T) {
	d := RecordedDriver{Samples: []TraceSample{{Lap: 10, Status: 4, Time: 1, Frame: 1, Distance: 10, LapMS: 1000}, {Lap: 10, Status: 2, Time: 2, Frame: 2, Distance: 4900, LapMS: 2000}, {Lap: 10, Status: 3, Time: 3, Frame: 3, Distance: 5100, LapMS: 3000}}}
	if len(d.traceRun(10, true)) != 3 || len(d.trace(10)) != 1 {
		t.Fatal("race pit-lane samples were filtered, or qualifying rules changed")
	}
	m := newModel(RecordingMeta{})
	for i, time := range []float64{1000, 1010, 990} {
		m.Drivers[i].Classified = true
		m.Drivers[i].Completed = 20
		m.Drivers[i].Position = i + 1
		m.Drivers[i].ResultStatus = 3
		m.Drivers[i].RaceSeconds = time
		m.Drivers[i].Laps[20] = &RecordedLap{Time: 90000, Valid: true}
	}
	m.Drivers[2].PenaltySeconds = 15
	metrics := m.metrics()
	if metrics.Laps[1][20].GapMS != 10000 || metrics.Laps[2][20].GapMS != 5000 {
		t.Fatal("finish gap does not use classified elapsed time plus penalties", metrics.Laps)
	}
	delete(m.Drivers[1].Laps, 20)
	m.raceMetrics = nil
	finish := m.metrics().Laps[1][20]
	if finish.GapMS != 10000 || finish.Position != 2 || finish.Time != 0 {
		t.Fatal("classification should supply finish position/gap without inventing an unavailable lap time", finish)
	}
	m.Drivers[0].Laps[21] = &RecordedLap{Time: 91000, Valid: true}
	m.raceMetrics = nil
	if m.metrics().MaxLap != 20 {
		t.Fatal("post-race lap included beyond final classified distance")
	}
	m.Drivers[1].Completed = 19
	m.raceMetrics = nil
	if m.metrics().Laps[1][20].Flags&flagGapKnown != 0 {
		t.Fatal("derived a gap across unequal race distances")
	}
	m.Drivers[0].Pits = []PitVisit{{Time: 10, LaneMS: 15000, StopMS: 2000, Complete: true}, {Time: 50, LaneMS: 17000, StopMS: 2500, Complete: true}}
	m.markDriveThrough(0, 24.5)
	avg, count := averagePitLane(&m.Drivers[0])
	if count != 1 || avg != 17 {
		t.Fatal("DTSV-marked drive-through remained in the pit average", avg, count)
	}
}
func TestLapProgressionDashesAndSummary(t *testing.T) {
	count := func(dashed bool) int {
		f := newRaceFigure(1000, 500, "", "")
		f.axes(1, 2, 0, 100, "", "", false)
		f.line(raceSeries{Driver: 0, Points: []racePoint{{X: 1, Y: 50}, {X: 2, Y: 50, Dashed: dashed}}}, true, false)
		n := 0
		for y := f.plot.Min.Y; y <= f.plot.Max.Y; y++ {
			for x := f.plot.Min.X; x <= f.plot.Max.X; x++ {
				if f.im.RGBAAt(x, y) == raceColours[0] {
					n++
				}
			}
		}
		return n
	}
	solid, dashed := count(false), count(true)
	if dashed >= solid*4/5 || dashed <= solid*2/5 {
		t.Fatal("pit connection should have visible dashed gaps", solid, dashed)
	}
	m := newModel(RecordingMeta{})
	m.Drivers[0].Laps[1] = &RecordedLap{Time: 60000, Valid: true}
	m.Drivers[0].Laps[2] = &RecordedLap{Time: 50000, Valid: false}
	m.Drivers[0].Laps[3] = &RecordedLap{Time: 100000, Valid: true}
	m.Drivers[0].Laps[4] = &RecordedLap{}
	best, average := raceLapSummary(m, 0)
	if best != 60000 || average != 70000 {
		t.Fatal("best valid / average all completed timed laps", best, average)
	}
}
func TestSectorDuelAcrossRace(t *testing.T) {
	m := newModel(RecordingMeta{})
	m.Drivers[0].Laps[1] = &RecordedLap{Time: 60000, S1: 10000, S2: 20000, S3: 30000, Valid: true}
	m.Drivers[1].Laps[1] = &RecordedLap{Time: 62000, S1: 11000, S2: 19000, S3: 32000, Valid: true}
	m.Drivers[0].Laps[2] = &RecordedLap{Time: 65000, S1: 12000, S2: 21000, S3: 32000, Valid: false}
	m.Drivers[1].Laps[2] = &RecordedLap{Time: 65000, S1: 12000, S2: 22000, S3: 31000, Valid: true}
	m.Drivers[0].Laps[3] = &RecordedLap{Time: 60000, S1: 10000, S2: 20000, S3: 30000, Valid: true}
	cells, totals := sectorDuelWinners(m, []int{0, 1})
	if !reflect.DeepEqual(cells[1], [3][]int{{0}, {1}, {0}}) || !reflect.DeepEqual(cells[2], [3][]int{{0, 1}, {0}, {1}}) {
		t.Fatal("sector winners should span laps, retain invalid laps and share ties", cells)
	}
	if len(cells[3][0]) != 0 || totals[0][0] != 2 || totals[0][1] != 1 || totals[1][0] != 1 || totals[1][1] != 1 {
		t.Fatal("missing comparisons must stay blank and totals must count every captured lap", cells, totals)
	}
}
func TestRaceV3ReplayFromEnvironment(t *testing.T) {
	path, out := os.Getenv("UDP_TELEMETRY_RACE_REPLAY"), os.Getenv("UDP_TELEMETRY_V3_PREVIEW")
	if path == "" || out == "" {
		t.Skip("no real race preview configured")
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
	if m.metrics().MaxLap != 20 {
		t.Fatal("race distance", m.metrics().MaxLap)
	}
	for i, d := range m.Drivers {
		if d.Name == "" {
			continue
		}
		count := 0
		missing := []int{}
		for lap, r := range m.metrics().Laps[i] {
			if r.Time > 0 && r.Flags&flagGapKnown != 0 {
				count++
			} else if r.Time > 0 {
				missing = append(missing, lap)
			}
		}
		t.Logf("%s: %d laps with recorded leader gaps; unavailable on %v", d.Name, count, missing)
		if count < 10 {
			t.Fatal("gap data missing for", d.Name, count)
		}
	}
	avg, n := averagePitLane(&m.Drivers[23])
	if n != 1 || math.Abs(avg-17.733) > .001 {
		t.Fatal("BROWNING full pit-lane mean", avg, n)
	}
	a := App{data: out, displayCache: filepath.Join(out, "launcher-cache"), rec: &Recorder{items: []RecordingMeta{meta}}, analysis: AnalysisSelection{Model: m, RaceSelectionSession: meta.ID, RaceDrivers: (1 << 23) | (1 << 1) | (1 << 6), RaceLap: 10}}
	for _, tab := range []string{"stats", "traces", "tyres", "positions", "progression"} {
		a.analysis.Tab = tab
		view := a.analysisView()
		b, _ = json.MarshalIndent(view, "", "  ")
		if e = os.WriteFile(filepath.Join(out, tab+".json"), b, 0644); e != nil {
			t.Fatal(e)
		}
		t.Logf("%s: %d images", tab, len(wireNodes(view, "image")))
	}
	t.Logf("BROWNING completed laps %d, full pit-lane average %.3fs, lap 10 flags %s", m.Drivers[23].Completed, avg, raceFlags(m.metrics().Laps[23][10].Flags))
	if os.Getenv("UDP_TELEMETRY_V3_ALL_DRIVERS") == "1" {
		a.analysis.RaceDrivers = 0
		for _, i := range raceDriverIndices(m) {
			a.analysis.RaceDrivers |= 1 << uint(i)
		}
		for _, tab := range []string{"traces", "tyres", "positions", "progression"} {
			a.analysis.Tab = tab
			start := time.Now()
			view := a.analysisView()
			b, _ = json.MarshalIndent(view, "", "  ")
			if e = os.WriteFile(filepath.Join(out, "all-"+tab+".json"), b, 0644); e != nil {
				t.Fatal(e)
			}
			t.Logf("Full-grid %s rendered in %s", tab, time.Since(start).Round(time.Millisecond))
		}
	}
}
