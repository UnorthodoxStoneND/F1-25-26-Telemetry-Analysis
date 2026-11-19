//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

func TestTelemetryAndBadgesCommitTogether(t *testing.T) {
	create := kernel.NewProc("CreateFileMappingW")
	write := func(at uintptr, b []byte) {
		t.Helper()
		var got uintptr
		ok, _, err := kernel.NewProc("WriteProcessMemory").Call(^uintptr(0), at, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&got)))
		if ok == 0 || got != uintptr(len(b)) {
			t.Fatal(err)
		}
	}
	mapping := func(size int, name *uint16) (uintptr, uintptr) {
		t.Helper()
		h, _, err := create.Call(^uintptr(0), 0, 4, 0, uintptr(size), uintptr(unsafe.Pointer(name)))
		if h == 0 {
			t.Fatal(err)
		}
		t.Cleanup(func() { syscall.CloseHandle(syscall.Handle(h)) })
		address, _, err := mapView.Call(h, 2, 0, 0, uintptr(size))
		if address == 0 {
			t.Fatal(err)
		}
		t.Cleanup(func() { unmapView.Call(address) })
		return h, address
	}
	mainHandle, mainAddress := mapping(frameSize, nil)
	pid := uint32(os.Getpid()) + 0x70000000
	name, _ := syscall.UTF16PtrFromString(fmt.Sprintf("Local\\USTelemetryBadges-2-%d", pid))
	_, badgeAddress := mapping(badgeFrameSize, name)
	r := Reader{pid: pid}
	var err error
	r.process, err = syscall.OpenProcess(0x100000, false, uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	r.address, _, _ = mapView.Call(mainHandle, 4, 0, 0, frameSize)
	if r.address == 0 {
		t.Fatal("read mapping failed")
	}
	t.Cleanup(r.Close)
	telemetry := fixture()
	binary.LittleEndian.PutUint64(telemetry[16:], 2)
	binary.LittleEndian.PutUint64(telemetry[24:], 42)
	binary.LittleEndian.PutUint64(telemetry[256+272:], 1234)
	binary.LittleEndian.PutUint32(telemetry[256+4:], knownName|knownPosition|knownTyre)
	binary.LittleEndian.PutUint32(telemetry[256+8:], 1)
	binary.LittleEndian.PutUint32(telemetry[256+16:], 18)
	badges := badgeFixture()
	binary.LittleEndian.PutUint64(badges[16:], 2)
	write(mainAddress, telemetry)
	write(badgeAddress, badges)
	first, err := r.Read()
	if err != nil || first.Cars[0].BadgePNG == "" || r.badgeAddress == 0 {
		t.Fatal("paired view or badge mapping failed", err)
	}
	oldName, oldBadge := first.Cars[0].Name, first.Cars[0].BadgePNG
	// Badges ahead of telemetry must never alter the last displayed frame.
	binary.LittleEndian.PutUint64(badges[16:], 4)
	binary.LittleEndian.PutUint64(badges[40:], 8)
	badges[32+280] = 20
	write(badgeAddress, badges)
	if _, err = r.Read(); !errors.Is(err, errUpdating) {
		t.Fatal("accepted badges from a newer generation", err)
	}
	if first.Cars[0].Name != oldName || first.Cars[0].BadgePNG != oldBadge {
		t.Fatal("incomplete read mutated the displayed frame")
	}
	binary.LittleEndian.PutUint64(telemetry[16:], 4)
	clear(telemetry[256+140 : 256+268])
	copy(telemetry[256+140:], "New Driver")
	write(mainAddress, telemetry)
	second, err := r.Read()
	if err != nil || second.Cars[0].Name != "New Driver" || second.Cars[0].BadgePNG == oldBadge {
		t.Fatal("matching generation did not update together", err)
	}
	// Telemetry ahead of badges is equally incomplete.
	binary.LittleEndian.PutUint64(telemetry[16:], 6)
	clear(telemetry[256+140 : 256+268])
	copy(telemetry[256+140:], "Future Driver")
	write(mainAddress, telemetry)
	if _, err = r.Read(); !errors.Is(err, errUpdating) {
		t.Fatal("accepted telemetry from a newer generation", err)
	}
	if second.Cars[0].Name != "New Driver" {
		t.Fatal("failed read changed the displayed driver")
	}
	binary.LittleEndian.PutUint64(badges[16:], 7)
	write(badgeAddress, badges)
	if _, err = r.Read(); !errors.Is(err, errUpdating) {
		t.Fatal("accepted an in-progress badge copy", err)
	}
	// End/unload must clear immediately, even with an unfinished badge buffer.
	binary.LittleEndian.PutUint64(telemetry[16:], 8)
	binary.LittleEndian.PutUint32(telemetry[40:], 32)
	binary.LittleEndian.PutUint32(telemetry[48:], 0)
	write(mainAddress, telemetry)
	ended, err := r.Read()
	if err != nil || len(ended.Cars) != 0 || len(r.badges.images) != 0 {
		t.Fatal("session clearing waited for badges", err)
	}
}

func TestBadgesWaitForRowTelemetry(t *testing.T) {
	a := App{frame: grid(), mode: "full", status: "Live"}
	c := &a.frame.Cars[0]
	c.Known &^= knownTyre
	a.lastView, a.viewSent = a.viewState(), true
	c.BadgePNG = testBadgePNG(t)
	if badgeCell(*c) != "" || a.needsView() {
		t.Fatal("badge appeared before row telemetry")
	}
	c.Known |= knownTyre
	if !a.needsView() || badgeCell(*c) == "" {
		t.Fatal("ready row did not show its badge with telemetry")
	}
	c.Known &^= knownName
	if badgeCell(*c) != "" {
		t.Fatal("badge appeared without a driver")
	}
}
