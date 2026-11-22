//go:build desktop

package main

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

type windowGeometry struct {
	window                   fyne.Window
	path                     string
	saved, candidate         windowPlacement
	haveSaved, haveCandidate bool
	// Normal bounds stay available when a window manager reports only maximized bounds.
	current windowPlacement
}

func (g *windowGeometry) prepare() func() {
	monitors, launcher := geometryEnvironment(os.Getppid())
	if len(monitors) == 0 {
		return func() {}
	}
	saved, exists := loadWindowPlacement(g.path)
	target := fitWindowPlacement(saved, exists, monitors, launcher)
	return geometryBeforeShow(g.window.Title(), target)
}

func (g *windowGeometry) native(fn func(uintptr)) {
	if native, ok := g.window.(driver.NativeWindow); ok {
		native.RunNative(func(context any) {
			if handle := geometryHandle(context); handle != 0 {
				fn(handle)
			}
		})
	}
}
func (g *windowGeometry) open() {
	g.native(func(handle uintptr) {
		monitors, launcher := geometryEnvironment(os.Getppid())
		saved, exists := loadWindowPlacement(g.path)
		if !exists && g.haveSaved {
			saved = g.saved
			exists = true
		}
		if len(monitors) == 0 {
			return
		}
		target := fitWindowPlacement(saved, exists, monitors, launcher)
		if geometryApply(handle, target) {
			g.current = target
		}
		g.saved = saved
		g.haveSaved = exists
	})
}
func (g *windowGeometry) capture(force bool) {
	g.native(func(handle uintptr) {
		next, normalKnown, ok := geometryRead(handle)
		if !ok {
			return
		}
		if err := g.observe(next, normalKnown, force); err != nil {
			fmt.Fprintln(os.Stderr, "Save retro window:", err)
		}
	})
}

// This ticker queues lightweight native geometry checks on the same UI thread
// as the window. Only a settled change writes preferences; no telemetry is saved.
func geometryPollInterval() time.Duration { return time.Second }

func (g *windowGeometry) observe(next windowPlacement, normalKnown, force bool) error {
	if next.Maximized && !normalKnown {
		if !validWindowRect(g.current.Bounds) {
			return nil
		}
		next.Bounds = g.current.Bounds
	}
	if !validWindowRect(next.Bounds) {
		return nil
	}
	next.Version = 1
	g.current = next
	if g.haveSaved && next == g.saved {
		g.haveCandidate = false
		return nil
	}
	if !force && (!g.haveCandidate || g.candidate != next) {
		g.candidate = next
		g.haveCandidate = true
		return nil
	}
	if err := saveWindowPlacement(g.path, next); err != nil {
		return err
	}
	g.saved = next
	g.haveSaved = true
	g.haveCandidate = false
	return nil
}
