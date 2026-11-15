package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScopedSavedReplayMatchesFull(t *testing.T) {
	dir := os.Getenv("UDP_TELEMETRY_SCOPED_REPLAY")
	if dir == "" {
		t.Skip("optional saved-session accuracy comparison")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	count := 0
	for _, path := range files {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var meta RecordingMeta
		if json.Unmarshal(b, &meta) != nil || meta.State != "completed" || meta.Session < 1 || meta.Session > 17 {
			continue
		}
		full, e := loadAnalysis(dir, meta)
		if e != nil {
			t.Fatal(e)
		}
		scopes := []analysisTraceScope{{}}
		if raceSession(meta.Session) {
			scopes = append(scopes, analysisTraceScope{Lap: 1, Drivers: (1 << 24) - 1}, analysisTraceScope{Lap: 2, Drivers: 1})
		}
		for _, scope := range scopes {
			m, e := loadAnalysisScoped(context.Background(), dir, meta, scope)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(full.metrics(), m.metrics()) || !reflect.DeepEqual(full.Conditions, m.Conditions) {
				t.Fatal("scoped replay changed race/conditions data", meta.Session, scope)
			}
			for i, d := range full.Drivers {
				got := m.Drivers[i]
				if !reflect.DeepEqual(d.Laps, got.Laps) || !reflect.DeepEqual(d.Pits, got.Pits) || !reflect.DeepEqual(d.Stints, got.Stints) || d.Position != got.Position || d.Grid != got.Grid || d.Best != got.Best || d.Completed != got.Completed {
					t.Fatal("scoped timing/strategy changed", meta.Session, scope, i)
				}
				n := 0
				for _, s := range d.Samples {
					if scope.captures(i, s.Lap) {
						if n >= len(got.Samples) || s != got.Samples[n] {
							t.Fatal("selected driving sample changed", meta.Session, scope, i, n)
						}
						n++
					}
				}
				if n != len(got.Samples) {
					t.Fatal("unexpected driving samples", meta.Session, scope, i)
				}
			}
		}
		count++
	}
	t.Logf("Compared scoped and original full replay for %d completed sessions: timing, strategy, conditions and selected driving samples match", count)
}

func scopedJournal(t *testing.T, format int) (string, RecordingMeta) {
	t.Helper()
	dir := t.TempDir()
	meta := RecordingMeta{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Format: format, Session: 15, State: "completed", Player: 0}
	f, e := os.Create(filepath.Join(dir, meta.ID+".udp.gz"))
	if e != nil {
		t.Fatal(e)
	}
	z := gzip.NewWriter(f)
	put := func(b []byte) {
		t.Helper()
		var size [2]byte
		binary.LittleEndian.PutUint16(size[:], uint16(len(b)))
		if _, e := z.Write(size[:]); e != nil {
			t.Fatal(e)
		}
		if _, e := z.Write(b); e != nil {
			t.Fatal(e)
		}
	}
	lights := eventAt(format, 1, 0, "STLG")
	lights[33] = 1
	put(lights)
	for lap := 1; lap <= 3; lap++ {
		for n := 0; n < 8; n++ {
			frame := uint32(10*lap + n)
			clock := float32(lap*10) + float32(n)/10
			lp, car := raceGridPair(format, frame, clock, uint32(n*100), float32(n*100), 100+lap)
			for i := 0; i < 24 && 29+(i+1)*57 <= len(lp); i++ {
				lp[29+i*57+33] = byte(lap)
			}
			if n%2 == 0 {
				put(car)
				put(lp)
			} else {
				put(lp)
				put(car)
			}
		}
	}
	flash := eventAt(format, 100, 31, "FLBK")
	binary.LittleEndian.PutUint32(flash[33:], 12)
	putFloat(flash[37:], 10.2)
	put(flash)
	lp, car := raceGridPair(format, 101, 10.3, 300, 300, 180)
	put(lp)
	put(car)
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	return dir, meta
}

func TestCompactLapMarkersPreserveReorderedBoundaries(t *testing.T) {
	d := RecordedDriver{}
	for block := 0; block < 32; block++ {
		for n := 31; n >= 0; n-- {
			frame := block*32 + n + 1
			d.noteTraceLap(TraceSample{Time: float64(frame) / 10, Frame: frame, Lap: frame/200 + 1})
		}
	}
	for frame := 1; frame <= 1024; frame++ {
		if got := d.traceLapAt(float64(frame) / 10); got != frame/200+1 {
			t.Fatal("reordered boundary changed", frame, got)
		}
	}
	if len(d.traceLaps) > 200 {
		t.Fatal("lap marker history is not compact", len(d.traceLaps))
	}
	d.trimTraceLaps(20.5, nil)
	if d.traceLapAt(20.5) != 2 {
		t.Fatal("flashback lap missing")
	}
	d = RecordedDriver{}
	for _, s := range []TraceSample{{Time: 1, Frame: 1, Lap: 1}, {Time: 10, Frame: 10, Lap: 2}, {Time: 15, Frame: 15, Lap: 1}, {Time: 12, Frame: 12, Lap: 1}, {Time: 14, Frame: 14, Lap: 2}} {
		d.noteTraceLap(s)
	}
	if d.traceLapAt(14.5) != 2 || d.traceLapAt(15) != 1 {
		t.Fatal("late sample erased a future boundary")
	}
}
func TestScopedReplayPreservesSelectedSamplesAndMetrics(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		dir, meta := scopedJournal(t, format)
		full, e := loadAnalysis(dir, meta)
		if e != nil {
			t.Fatal(e)
		}
		for _, scope := range []analysisTraceScope{{}, {Lap: 1, Drivers: 1}, {Lap: 2, Drivers: (1 << 24) - 1}} {
			scoped, e := loadAnalysisScoped(context.Background(), dir, meta, scope)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(full.metrics(), scoped.metrics()) || full.Frames != scoped.Frames {
				t.Fatal("timing, position, gaps, flags or packet counts changed", format, scope)
			}
			for i, d := range scoped.Drivers {
				expected := []TraceSample{}
				for _, s := range full.Drivers[i].Samples {
					if scope.captures(i, s.Lap) {
						expected = append(expected, s)
					}
				}
				if len(expected) != len(d.Samples) {
					t.Fatal("wrong selected sample count", format, scope, i, len(expected), len(d.Samples))
				}
				for n, s := range d.Samples {
					if s != expected[n] {
						t.Fatal("selected sample changed", format, scope, i, n)
					}
				}
			}
		}
	}
}
func TestScopeChangesRequireReplayAndReleaseUnusedTraces(t *testing.T) {
	m := newModel(RecordingMeta{})
	scope := analysisTraceScope{Lap: 1, Drivers: 3}
	m.traceScope = &scope
	m.Drivers[0].Samples = []TraceSample{{Lap: 1}}
	m.Drivers[1].Samples = []TraceSample{{Lap: 1}}
	if !m.coversScope(analysisTraceScope{Lap: 1, Drivers: 1}) || m.coversScope(analysisTraceScope{Lap: 2, Drivers: 1}) || m.coversScope(analysisTraceScope{Lap: 1, Drivers: 7}) {
		t.Fatal("scope coverage")
	}
	m.releaseUnusedTraces(analysisTraceScope{Lap: 1, Drivers: 1})
	if len(m.Drivers[0].Samples) != 1 || m.Drivers[1].Samples != nil || m.coversScope(scope) {
		t.Fatal("deselected trace retained")
	}
	m.releaseUnusedTraces(analysisTraceScope{})
	if m.Drivers[0].Samples != nil || m.traceScope.hasTraces() {
		t.Fatal("non-trace tab retained driving samples")
	}
	a := analysisFixture(15)
	a.analysis.Tab = "traces"
	a.analysis.RaceLap = 2
	a.ensureRaceSelection(a.analysis.Model)
	a.analysis.RaceLap = 2
	a.analysis.RaceDrivers = 3
	if got := a.wantedTraceScope(a.analysis.Model.Meta); got.Lap != 2 || got.Drivers != 3 {
		t.Fatal(got)
	}
	for _, tab := range []string{"stats", "tyres", "positions", "progression"} {
		a.analysis.Tab = tab
		if a.wantedTraceScope(a.analysis.Model.Meta).hasTraces() {
			t.Fatal("non-trace tab requested driving samples", tab)
		}
	}
}
func TestBorrowedJournalBufferAndCancellation(t *testing.T) {
	dir, meta := scopedJournal(t, 2026)
	path := filepath.Join(dir, meta.ID+".udp.gz")
	owned := [][]byte{}
	_, e := readJournal(path, func(b []byte) { owned = append(owned, b) })
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	_, e = readJournalContext(context.Background(), path, true, func(b []byte) {
		if !bytes.Equal(b, owned[n]) {
			t.Fatal("borrowed packet changed", n)
		}
		n++
	})
	if e != nil || n != len(owned) {
		t.Fatal(e, n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	n = 0
	_, e = readJournalContext(ctx, path, true, func([]byte) { n++; cancel() })
	if !errors.Is(e, context.Canceled) || n != 1 {
		t.Fatal("superseded replay did not stop", e, n)
	}
}
func TestUnchangedAnalysisDoesNotInvalidateEveryTick(t *testing.T) {
	a := analysisFixture(15)
	a.analysisView()
	if a.analysisNeedsInvalidate() {
		t.Fatal("unchanged page would be regenerated")
	}
	a.rec.items[0].Packets++
	if a.analysisNeedsInvalidate() {
		t.Fatal("completed snapshot churns for unseen late packets; Refresh loads a new snapshot")
	}
	a.rec.items[0].State = "recording"
	if !a.analysisNeedsInvalidate() {
		t.Fatal("late data not discovered")
	}
	a.analysisView()
	if a.analysisNeedsInvalidate() {
		t.Fatal("latest view stamp not stored")
	}
	a.rec.items[0].Packets++
	if !a.analysisNeedsInvalidate() {
		t.Fatal("active recording not discovered")
	}
	a.analysisView()
	a.rec.items[0].Save = "Renamed"
	if !a.analysisNeedsInvalidate() {
		t.Fatal("rename not discovered")
	}
}
