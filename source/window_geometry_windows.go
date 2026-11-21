//go:build desktop && windows

package main

import (
	"sync"
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

var geometryUser = syscall.NewLazyDLL("user32.dll")
var geometryEnumMonitors = geometryUser.NewProc("EnumDisplayMonitors")
var geometryMonitorInfo = geometryUser.NewProc("GetMonitorInfoW")
var geometryMonitorWindow = geometryUser.NewProc("MonitorFromWindow")
var geometryMonitorRect = geometryUser.NewProc("MonitorFromRect")
var geometryEnumWindows = geometryUser.NewProc("EnumWindows")
var geometryWindowPID = geometryUser.NewProc("GetWindowThreadProcessId")
var geometryVisible = geometryUser.NewProc("IsWindowVisible")
var geometryIconic = geometryUser.NewProc("IsIconic")
var geometryWindowRect = geometryUser.NewProc("GetWindowRect")
var geometryOwner = geometryUser.NewProc("GetWindow")
var geometryGetPlacement = geometryUser.NewProc("GetWindowPlacement")
var geometrySetPlacement = geometryUser.NewProc("SetWindowPlacement")

type geometryWinRect struct{ Left, Top, Right, Bottom int32 }
type geometryWinPoint struct{ X, Y int32 }
type geometryWinPlacement struct {
	Length, Flags, ShowCmd uint32
	Min, Max               geometryWinPoint
	Normal                 geometryWinRect
}
type geometryWinMonitor struct {
	Size          uint32
	Monitor, Work geometryWinRect
	Flags         uint32
}
type geometryMonitorEntry struct {
	handle uintptr
	work   windowRect
}
type geometryMonitorList struct{ entries []geometryMonitorEntry }
type geometryLauncherWindow struct {
	pid    uint32
	area   int64
	handle uintptr
}

// Enumerations are synchronous and serialized. Keep callback collectors in Go
// rather than passing Go pointers through an opaque native integer parameter.
var geometryEnumerationLock sync.Mutex
var geometryMonitorCollector *geometryMonitorList
var geometryLauncherCollector *geometryLauncherWindow

func (r geometryWinRect) rect() windowRect {
	return windowRect{int(r.Left), int(r.Top), int(r.Right - r.Left), int(r.Bottom - r.Top)}
}
func geometryWinBounds(r windowRect) geometryWinRect {
	return geometryWinRect{int32(r.X), int32(r.Y), int32(r.X + r.W), int32(r.Y + r.H)}
}
func geometryHandle(context any) uintptr {
	if native, ok := context.(driver.WindowsWindowContext); ok {
		return native.HWND
	}
	return 0
}
func geometryWinInfo(handle uintptr) geometryWinMonitor {
	info := geometryWinMonitor{}
	info.Size = uint32(unsafe.Sizeof(info))
	geometryMonitorInfo.Call(handle, uintptr(unsafe.Pointer(&info)))
	return info
}

var geometryMonitorCallback = syscall.NewCallback(func(monitor, dc, rect, data uintptr) uintptr {
	list := geometryMonitorCollector
	if list == nil {
		return 0
	}
	if len(list.entries) >= 64 {
		return 0
	}
	info := geometryWinInfo(monitor)
	r := info.Work.rect()
	if r.W > 0 && r.H > 0 {
		list.entries = append(list.entries, geometryMonitorEntry{monitor, r})
	}
	return 1
})
var geometryWindowCallback = syscall.NewCallback(func(window, data uintptr) uintptr {
	parent := geometryLauncherCollector
	if parent == nil {
		return 0
	}
	var pid uint32
	geometryWindowPID.Call(window, uintptr(unsafe.Pointer(&pid)))
	if pid != parent.pid {
		return 1
	}
	visible, _, _ := geometryVisible.Call(window)
	owner, _, _ := geometryOwner.Call(window, 4)
	if visible == 0 || owner != 0 {
		return 1
	}
	var rect geometryWinRect
	if ok, _, _ := geometryWindowRect.Call(window, uintptr(unsafe.Pointer(&rect))); ok != 0 {
		area := int64(rect.Right-rect.Left) * int64(rect.Bottom-rect.Top)
		if area > parent.area {
			parent.area = area
			parent.handle = window
		}
	}
	return 1
})

func geometryEnvironment(parentPID int) ([]windowRect, int) {
	geometryEnumerationLock.Lock()
	defer geometryEnumerationLock.Unlock()
	list := geometryMonitorList{}
	geometryMonitorCollector = &list
	geometryEnumMonitors.Call(0, 0, geometryMonitorCallback, 0)
	geometryMonitorCollector = nil
	parent := geometryLauncherWindow{pid: uint32(parentPID)}
	geometryLauncherCollector = &parent
	geometryEnumWindows.Call(geometryWindowCallback, 0)
	geometryLauncherCollector = nil
	chosen, _, _ := geometryMonitorWindow.Call(parent.handle, 2)
	monitors := make([]windowRect, 0, len(list.entries))
	preferred := 0
	for i, entry := range list.entries {
		monitors = append(monitors, entry.work)
		if entry.handle == chosen {
			preferred = i
		}
	}
	return monitors, preferred
}
func geometryRead(handle uintptr) (windowPlacement, bool, bool) {
	visible, _, _ := geometryVisible.Call(handle)
	iconic, _, _ := geometryIconic.Call(handle)
	if visible == 0 || iconic != 0 {
		return windowPlacement{}, false, false
	}
	p := geometryWinPlacement{}
	p.Length = uint32(unsafe.Sizeof(p))
	if ok, _, _ := geometryGetPlacement.Call(handle, uintptr(unsafe.Pointer(&p))); ok == 0 {
		return windowPlacement{}, false, false
	}
	monitor, _, _ := geometryMonitorWindow.Call(handle, 2)
	info := geometryWinInfo(monitor)
	// WINDOWPLACEMENT uses workspace coordinates. Persist desktop coordinates
	// so monitor/taskbar changes can be clamped before converting back on restore.
	b := p.Normal.rect()
	b.X += int(info.Work.Left - info.Monitor.Left)
	b.Y += int(info.Work.Top - info.Monitor.Top)
	return windowPlacement{Version: 1, Bounds: b, Maximized: p.ShowCmd == 3}, true, true
}
func geometryApply(handle uintptr, saved windowPlacement) bool {
	rect := geometryWinBounds(saved.Bounds)
	monitor, _, _ := geometryMonitorRect.Call(uintptr(unsafe.Pointer(&rect)), 2)
	info := geometryWinInfo(monitor)
	b := saved.Bounds
	b.X -= int(info.Work.Left - info.Monitor.Left)
	b.Y -= int(info.Work.Top - info.Monitor.Top)
	p := geometryWinPlacement{ShowCmd: 4, Normal: geometryWinBounds(b)}
	p.Length = uint32(unsafe.Sizeof(p))
	if saved.Maximized {
		p.ShowCmd = 3
	}
	ok, _, _ := geometrySetPlacement.Call(handle, uintptr(unsafe.Pointer(&p)))
	return ok != 0
}
