package main

import (
	"compress/gzip"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSprintWeekendIdentityAndLegacyJournal(t *testing.T) {
	structure := []int{1, 13, 15, 8, 16}
	for _, format := range []int{2025, 2026} {
		p := packet(format, 1, 123, 1)
		p[29+703] = 5
		for i, kind := range structure {
			p[29+704+i] = byte(kind)
		}
		if got := packetWeekendStructure(p[29:]); !reflect.DeepEqual(got, structure) {
			t.Fatal(got)
		}
		for _, check := range []struct {
			kind  int
			label string
		}{{15, "Sprint"}, {16, "Race"}, {13, "Short sprint qualifying"}} {
			if got := recordingSessionLabel(RecordingMeta{Session: check.kind, WeekendStructure: structure}); got != check.label {
				t.Fatal(got, check.label)
			}
		}
		data := t.TempDir()
		dir := filepath.Join(data, "recordings")
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		meta := RecordingMeta{ID: strings.Repeat("a", 32), Session: 15, State: "completed"}
		if e := atomicJSON(filepath.Join(dir, meta.ID+".json"), meta); e != nil {
			t.Fatal(e)
		}
		f, e := os.Create(filepath.Join(dir, meta.ID+".udp.gz"))
		if e != nil {
			t.Fatal(e)
		}
		z := gzip.NewWriter(f)
		var size [2]byte
		binary.LittleEndian.PutUint16(size[:], uint16(len(p)))
		_, _ = z.Write(size[:])
		_, _ = z.Write(p)
		_ = z.Close()
		_ = f.Close()
		r, e := openRecorder(data)
		if e != nil {
			t.Fatal(e)
		}
		if len(r.items) != 1 || recordingSessionLabel(r.items[0]) != "Sprint" {
			t.Fatal("old recording schedule not recovered", r.items)
		}
	}
	for _, schedule := range [][]int{nil, {1, 8, 15}, {1, 8, 15, 16}, {1, 13, 15, 8, 15}} {
		if recordingSessionLabel(RecordingMeta{Session: 15, WeekendStructure: schedule}) != "Race" {
			t.Fatal("ambiguous/non-sprint schedule mislabeled", schedule)
		}
	}
	if path := os.Getenv("UDP_TELEMETRY_SPRINT_JOURNAL"); path != "" {
		got := journalWeekendStructure(path)
		if recordingSessionLabel(RecordingMeta{Session: 15, WeekendStructure: got}) != "Sprint" {
			t.Fatal("saved Sprint", got)
		}
		t.Log("Saved Sprint schedule", got)
	}
}
