//go:build desktop

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowGeometryDebounceAndMaximize(t *testing.T) {
	path := windowPreferencePath(t.TempDir())
	g := windowGeometry{path: path}
	first := windowPlacement{Version: 1, Bounds: windowRect{2100, 80, 1300, 800}}
	if err := g.observe(first, true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("wrote geometry before settling")
	}
	moved := first
	moved.Bounds.X++
	g.observe(moved, true, false)
	if _, ok := loadWindowPlacement(path); ok {
		t.Fatal("wrote while still moving")
	}
	if err := g.observe(moved, true, false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	g.observe(moved, true, false)
	after, _ := os.ReadFile(path)
	if string(raw) != string(after) || g.haveCandidate {
		t.Fatal("idle window changed preferences")
	}
	maximized := windowPlacement{Version: 1, Bounds: windowRect{1920, 0, 1920, 1080}, Maximized: true}
	if err := g.observe(maximized, false, true); err != nil {
		t.Fatal(err)
	}
	saved, ok := loadWindowPlacement(path)
	if !ok || saved.Bounds != moved.Bounds || !saved.Maximized {
		t.Fatal("maximized capture lost normal bounds")
	}
	restored := moved
	restored.Bounds.W = 1400
	g.observe(restored, true, true)
	saved, ok = loadWindowPlacement(path)
	if !ok || saved != restored {
		t.Fatal("hide/close did not flush latest layout")
	}
	// A failed save must not be marked successful or destroy the last good file.
	g.path = filepath.Join(path, "not-a-directory.json")
	if err := g.observe(first, true, true); err == nil {
		t.Fatal("save failure swallowed")
	}
	if g.saved != restored {
		t.Fatal("failed save replaced last saved state")
	}
}
