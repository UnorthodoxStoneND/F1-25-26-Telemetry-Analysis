package main

import (
	"encoding/binary"
	"math"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRecordTelemetryAfterResultsAndEnd(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		t.Run(strconv.Itoa(format), func(t *testing.T) {
			r, e := openRecorder(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			defer r.finish("interrupted")
			r.ingest(lapAt(format, 1, 1, 1, 100), "p")
			r.ingest(traceAt(format, 1, 1, 201), "p")
			id := r.current.ID
			results := analysisPacket(format, 8, 2, 2)
			results[29] = 1
			results[30], results[31], results[32] = 1, 1, 3
			binary.LittleEndian.PutUint32(results[37:], 90000)
			binary.LittleEndian.PutUint64(results[41:], math.Float64bits(90))
			r.ingest(results, "p")
			r.ingest(lapAt(format, 3, 3, 1, 200), "p")
			r.ingest(traceAt(format, 3, 3, 202), "p")
			end := eventAt(format, 4, 4, "SEND")
			if format == 2025 {
				binary.LittleEndian.PutUint64(end[7:], 0)
			}
			r.ingest(end, "p")
			r.ingest(traceAt(format, 5, 5, 203), "p")
			r.ingest(lapAt(format, 5, 5, 1, 300), "p")
			if r.current.ID != id || r.current.State != "completed" || r.current.TelemetryAfterClassification != 2 || r.current.TelemetryAfterEnd != 1 || r.current.LastTelemetryTime == nil || *r.current.LastTelemetryTime != 5 {
				t.Fatalf("late diagnostics %+v", r.current)
			}
			if e = r.flush(); e != nil {
				t.Fatal(e)
			}
			m, e := loadAnalysis(r.dir, *r.current)
			if e != nil || m.Truncated {
				t.Fatal("late journal", e, m.Truncated)
			}
			samples := m.Drivers[0].trace(1)
			if len(samples) != 3 || samples[2].Speed != 203 {
				t.Fatalf("late traces were cut off: %+v", samples)
			}
			d := m.Drivers[0]
			if d.Position != 1 || d.Grid != 3 || d.Completed != 1 || d.Best != 90000 || d.RaceSeconds != 90 {
				t.Fatalf("late lap data overwrote classification: %+v", d)
			}
			// An explicit new UID closes capture. Old late packets cannot contaminate it.
			next := lapAt(format, 6, 0, 1, 1)
			binary.LittleEndian.PutUint64(next[7:], 456)
			r.ingest(next, "p")
			nextID := r.current.ID
			before := r.current.Packets
			r.ingest(traceAt(format, 7, 6, 204), "p")
			if r.current.ID != nextID || r.current.Packets != before {
				t.Fatal("retired UID contaminated next session")
			}
			old, err := loadAnalysis(r.dir, r.items[0])
			if err != nil || old.Meta.ID != id || len(old.Drivers[0].trace(1)) != 3 {
				t.Fatal("previous completed recording changed", err)
			}
		})
	}
}

func TestMenuStillClosesCompletedCapture(t *testing.T) {
	r, e := openRecorder(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s := analysisPacket(2026, 1, 1, 0)
	s[35], s[36] = 8, 0
	r.ingest(s, "p")
	r.ingest(eventAt(2026, 2, 1, "SEND"), "p")
	id := r.current.ID
	menu := analysisPacket(2026, 1, 3, 0)
	binary.LittleEndian.PutUint64(menu[7:], 0)
	menu[35], menu[36] = 0, 255
	r.ingest(menu, "p")
	if r.current != nil || len(r.items) != 1 || r.items[0].State != "completed" {
		t.Fatal("menu did not close completed journal")
	}
	r.ingest(traceAt(2026, 4, 2, 200), "p")
	if r.current != nil {
		t.Fatal("menu-closed recording revived")
	}
	partial, e := readJournal(filepath.Join(r.dir, id+".udp.gz"), func([]byte) {})
	if partial || e != nil {
		t.Fatal("menu close footer", partial, e)
	}
}

func TestLateLapPacketsPreserveHistoryTimes(t *testing.T) {
	m := newModel(RecordingMeta{Format: 2026})
	history := analysisPacket(2026, 11, 1, 1)
	history[30] = 1
	binary.LittleEndian.PutUint32(history[36:], 90123)
	binary.LittleEndian.PutUint16(history[40:], 30000)
	binary.LittleEndian.PutUint16(history[43:], 30123)
	binary.LittleEndian.PutUint16(history[46:], 30000)
	history[49] = 15
	m.ingest(history)
	late := lapAt(2026, 2, 2, 2, 100)
	binary.LittleEndian.PutUint32(late[29:], 80000)
	m.ingest(late)
	l := m.Drivers[0].Laps[1]
	if l.Time != 90123 || l.S1 != 30000 || l.S2 != 30123 || l.S3 != 30000 {
		t.Fatalf("late live timing overwrote session history %+v", l)
	}
	// History marks the current partial lap valid but its full time is zero.
	// That placeholder must not hide a subsequently observed completed time.
	l.Time = 0
	m.ingest(late)
	if l.Time != 80000 {
		t.Fatal("partial history blocked lap completion", l.Time)
	}
}
