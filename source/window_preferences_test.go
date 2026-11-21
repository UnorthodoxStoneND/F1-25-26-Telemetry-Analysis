package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowPlacementMonitorSelection(t *testing.T) {
	work := []windowRect{{0, 0, 1920, 1040}, {1920, -400, 2560, 1400}, {-1600, 0, 1600, 900}}
	first := fitWindowPlacement(windowPlacement{}, false, work, 1)
	if first.Bounds != (windowRect{2400, -160, 1600, 920}) {
		t.Fatalf("first window not centred on launcher monitor: %+v", first)
	}
	saved := windowPlacement{Version: 1, Bounds: windowRect{-1500, 50, 1200, 800}}
	if restored := fitWindowPlacement(saved, true, work, 1); restored != saved {
		t.Fatal("saved monitor overridden by launcher monitor")
	}
	gone := fitWindowPlacement(saved, true, work[:2], 1)
	if intersectionArea(gone.Bounds, work[1]) != gone.Bounds.W*gone.Bounds.H {
		t.Fatal("removed monitor left window offscreen")
	}
	oversized := fitWindowPlacement(windowPlacement{Version: 1, Bounds: windowRect{1900, -900, 4000, 3000}}, true, work, 0)
	if intersectionArea(oversized.Bounds, work[1]) != oversized.Bounds.W*oversized.Bounds.H {
		t.Fatal("oversized window not fitted to current work area")
	}
}
func TestWindowPlacementPersistenceAndInvalidData(t *testing.T) {
	data := filepath.Join(t.TempDir(), "addon-data", addonID)
	path := windowPreferencePath(data)
	p := windowPlacement{Version: 1, Bounds: windowRect{-1200, 50, 1100, 700}, Maximized: true}
	if err := saveWindowPlacement(path, p); err != nil {
		t.Fatal(err)
	}
	if loaded, ok := loadWindowPlacement(windowPreferencePath(data)); !ok || loaded != p {
		t.Fatal("geometry did not survive helper restart")
	}
	// Installation/version changes do not participate in the preference path.
	if filepath.Base(path) != "retro-window.json" || filepath.Dir(path) != data {
		t.Fatal("geometry stored with installed/versioned files")
	}
	p.Bounds.X = 200
	if err := saveWindowPlacement(path, p); err != nil {
		t.Fatal(err)
	}
	if loaded, ok := loadWindowPlacement(path); !ok || loaded != p {
		t.Fatal("atomic replacement failed")
	}
	for _, raw := range []string{`{`, `{"version":9}`, `{"version":1,"bounds":{"X":0,"Y":0,"W":-1,"H":800}}`, `{"version":1,"bounds":{"X":9999999,"Y":0,"W":1200,"H":800}}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, ok := loadWindowPlacement(path); ok {
			t.Fatal("invalid geometry accepted")
		}
	}
	if windowPreferencePath("") != "" {
		t.Fatal("empty data directory wrote into working directory")
	}
}
