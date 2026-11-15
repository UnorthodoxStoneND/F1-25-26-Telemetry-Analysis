package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSummaryDisplayCacheWithDocumentsStorage(t *testing.T) {
	root := t.TempDir()
	a := App{data: filepath.Join(root, "Documents", "F1 Game Telemetry"), displayCache: filepath.Join(root, "Swofty", "addon-cache", "unorthodoxstonendmodding.udp-telemetry", "telemetry-display-v1")}
	m := newModel(RecordingMeta{Session: 15, Track: 16})
	m.Conditions = &SessionConditions{Track: 16, Air: 24, Surface: 45, Weather: 3, TotalLaps: 34}
	block := a.summaryBlock(m)
	spec, ok := block["image"].(M)
	if !ok {
		t.Fatal("summary not rendered", block)
	}
	path := spec["path"].(string)
	if !strings.HasPrefix(path, a.displayCache+string(os.PathSeparator)) {
		t.Fatal("launcher would reject Documents image", path)
	}
	originals, _ := filepath.Glob(filepath.Join(a.data, "session-summary-v1", "*.png"))
	if len(originals) != 1 {
		t.Fatal(originals)
	}
	original, e := os.ReadFile(originals[0])
	if e != nil {
		t.Fatal(e)
	}
	cached, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(cached, original) {
		t.Fatal("display copy differs from the original summary", e)
	}
	if e = os.Remove(path); e != nil {
		t.Fatal(e)
	}
	if a.summaryBlock(m)["image"].(M)["path"] != path {
		t.Fatal("cache recreation changed image identity")
	}
	if e = os.WriteFile(path, []byte("broken cache"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = a.displayImage(originals[0]); e != nil {
		t.Fatal(e)
	}
	cached, _ = os.ReadFile(path)
	if !bytes.Equal(cached, original) {
		t.Fatal("corrupt cache not repaired")
	}
	// If the cache is unwritable, retain the six values as native stats.
	blocked := filepath.Join(root, "blocked")
	if e = os.WriteFile(blocked, []byte("file"), 0644); e != nil {
		t.Fatal(e)
	}
	a.displayCache = filepath.Join(blocked, "cache")
	if stats, ok := a.summaryBlock(m)["stats"].([]M); !ok || len(stats) != 6 {
		t.Fatal("failed cache left a blank overview bar")
	}
}
func TestDisplayCacheContentChangesInvalidateImage(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "Documents", "chart.png")
	if e := os.MkdirAll(filepath.Dir(source), 0755); e != nil {
		t.Fatal(e)
	}
	a := App{displayCache: filepath.Join(root, "cache")}
	if e := os.WriteFile(source, []byte("first image"), 0644); e != nil {
		t.Fatal(e)
	}
	first, e := a.displayImage(source)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(source, []byte("updated image"), 0644); e != nil {
		t.Fatal(e)
	}
	second, e := a.displayImage(source)
	if e != nil || second == first {
		t.Fatal("launcher could reuse stale pixels", e)
	}
}
