//go:build desktop && linux

package main

import (
	"os"
	"strconv"
	"testing"
)

func TestNativeX11Geometry(t *testing.T) {
	raw := os.Getenv("RETRO_X11_TEST_WINDOW")
	if raw == "" {
		t.Skip("isolated X11 fixture not running")
	}
	handle, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	parent, _ := strconv.Atoi(os.Getenv("RETRO_X11_TEST_PARENT"))
	monitors, preferred := geometryEnvironment(parent)
	if len(monitors) == 0 || preferred < 0 || preferred >= len(monitors) {
		t.Fatal("X11 monitor/launcher lookup failed")
	}
	wanted := windowPlacement{Version: 1, Bounds: windowRect{120, 140, 800, 600}}
	if !geometryApply(uintptr(handle), wanted) {
		t.Fatal("X11 move/resize failed")
	}
	captured, normal, ok := geometryRead(uintptr(handle))
	if !ok || !normal || captured != wanted {
		t.Fatalf("X11 geometry mismatch: %+v", captured)
	}
	path := windowPreferencePath(t.TempDir())
	g := windowGeometry{path: path}
	if err := g.observe(captured, normal, true); err != nil {
		t.Fatal(err)
	}
	saved, ok := loadWindowPlacement(path)
	if !ok || saved != wanted {
		t.Fatal("X11 geometry persistence failed")
	}
	wanted.Bounds = windowRect{200, 250, 700, 500}
	geometryApply(uintptr(handle), wanted)
	if !geometryApply(uintptr(handle), saved) {
		t.Fatal("X11 geometry restore failed")
	}
	restored, _, ok := geometryRead(uintptr(handle))
	if !ok || restored != saved {
		t.Fatal("X11 restored different bounds")
	}
}
