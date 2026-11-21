//go:build desktop && windows

package main

import (
	"os"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"unsafe"
)

func TestWindowPlacementBeforeVisibility(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	create := geometryUser.NewProc("CreateWindowExW")
	destroy := geometryUser.NewProc("DestroyWindow")
	class, _ := syscall.UTF16PtrFromString("STATIC")
	title := "Retro hidden placement fixture"
	target := windowPlacement{Version: 1, Bounds: windowRect{340, 180, 800, 600}}
	finish := geometryBeforeShow(title, target)
	defer finish()
	makeHidden := func(name string) geometryWinRect {
		t.Helper()
		wide, _ := syscall.UTF16PtrFromString(name)
		// No WS_VISIBLE: these test windows never appear or take focus.
		handle, _, err := create.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(wide)), 0x00cf0000, 20, 30, 400, 300, 0, 0, 0, 0)
		if handle == 0 {
			t.Fatal(err)
		}
		defer destroy.Call(handle)
		visible, _, _ := geometryVisible.Call(handle)
		if visible != 0 {
			t.Fatal("fixture unexpectedly visible")
		}
		var rect geometryWinRect
		if ok, _, _ := geometryWindowRect.Call(handle, uintptr(unsafe.Pointer(&rect))); ok == 0 {
			t.Fatal("cannot read fixture bounds")
		}
		return rect
	}
	if got := makeHidden("Unrelated hidden fixture").rect(); got != (windowRect{20, 30, 400, 300}) {
		t.Fatal("creation hook affected another window")
	}
	if got := makeHidden(title).rect(); got != target.Bounds {
		t.Fatalf("window not positioned before visibility: %+v", got)
	}
	finish()
	if got := makeHidden(title).rect(); got != (windowRect{20, 30, 400, 300}) {
		t.Fatal("creation hook was not removed")
	}
}

func TestLauncherMonitorProbe(t *testing.T) {
	raw := os.Getenv("RETRO_LAUNCHER_TEST_PID")
	if raw == "" {
		t.Skip("explicit read-only launcher probe not requested")
	}
	pid, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatal(err)
	}
	monitors, preferred := geometryEnvironment(pid)
	t.Logf("launcher pid=%d monitors=%+v preferred=%d", pid, monitors, preferred)
	if len(monitors) > 0 {
		t.Logf("first-open bounds=%+v", fitWindowPlacement(windowPlacement{}, false, monitors, preferred).Bounds)
	}
}

func TestNativeWindowsMonitorEnumeration(t *testing.T) {
	// Read-only: no native window is created, moved or activated by this test.
	monitors, preferred := geometryEnvironment(os.Getpid())
	if len(monitors) == 0 {
		t.Skip("no interactive desktop monitors available")
	}
	if preferred < 0 || preferred >= len(monitors) {
		t.Fatal("invalid preferred monitor")
	}
	for _, work := range monitors {
		if work.W <= 0 || work.H <= 0 {
			t.Fatal("invalid monitor work area")
		}
	}
	again, _ := geometryEnvironment(os.Getpid())
	if len(again) != len(monitors) {
		t.Fatal("enumeration collectors retained stale monitors")
	}
}
