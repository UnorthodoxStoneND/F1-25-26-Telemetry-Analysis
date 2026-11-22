//go:build desktop

package main

import (
	"fmt"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.01 }

func TestRetroSlideRetargetAndLiveValues(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	model := performanceModel(t)
	screen := &retroScreen{bridge: newDesktopBridge()}
	root := screen.content(model)
	window := test.NewWindow(root)
	defer window.Close()
	window.Resize(fyne.NewSize(1600, 920))
	if screen.slides.animation != nil {
		t.Fatal("initial grid animated")
	}
	starts := 0
	screen.slides.start = func(*fyne.Animation) { starts++ }
	firstID, secondID := model.Rows[0].ID, model.Rows[1].ID
	first, second := screen.rows[firstID].node, screen.rows[secondID].node
	firstY, secondY := first.Position().Y, second.Position().Y
	model.Rows[0], model.Rows[1] = model.Rows[1], model.Rows[0]
	if screen.content(model) != root || starts != 1 || screen.slides.animation == nil {
		t.Fatal("reorder did not start one retained-row animation")
	}
	old := screen.slides.animation
	if old.Duration != retroSlideDuration || first.Position().Y != firstY || second.Position().Y != secondY {
		t.Fatal("rows jumped before the first animation frame")
	}
	old.Tick(0.5)
	if !near(first.Position().Y, (firstY+secondY)/2) || !near(second.Position().Y, (firstY+secondY)/2) {
		t.Fatal("rows did not interpolate")
	}
	for i, col := range model.Columns {
		if col.Key == "last" {
			model.Rows[1].Cells[i].Text = "1:23.456"
		}
	}
	screen.content(model)
	if starts != 1 || screen.slides.animation != old {
		t.Fatal("live values restarted the slide")
	}
	for i, col := range model.Columns {
		if col.Key == "last" && screen.rows[firstID].cells[i].text.Text != "1:23.456" {
			t.Fatal("live timing froze during slide")
		}
	}
	at := first.Position().Y
	model.Rows[0], model.Rows[1] = model.Rows[1], model.Rows[0]
	screen.content(model)
	replacement := screen.slides.animation
	if starts != 2 || replacement == nil || replacement == old || first.Position().Y != at {
		t.Fatal("retarget jumped or reused an obsolete transition")
	}
	old.Tick(1)
	if first.Position().Y != at {
		t.Fatal("cancelled frame moved a retargeted row")
	}
	replacement.Tick(0.5)
	if !near(first.Position().Y, (at+firstY)/2) {
		t.Fatal("replacement did not start from the displayed position")
	}
	replacement.Tick(1)
	if screen.slides.animation != nil || first.Position().Y != firstY || second.Position().Y != secondY {
		t.Fatal("slide did not settle in exact slots")
	}
}

func TestRetroSlideLifecycleResizeAndBounds(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	model := performanceModel(t)
	screen := &retroScreen{bridge: newDesktopBridge()}
	window := test.NewWindow(screen.content(model))
	defer window.Close()
	window.Resize(fyne.NewSize(1600, 920))
	screen.slides.start = func(*fyne.Animation) {}
	for i := 0; i < 2000; i++ {
		model.Rows[0], model.Rows[23] = model.Rows[23], model.Rows[0]
		screen.content(model)
		if animation := screen.slides.animation; animation != nil {
			animation.Tick(0.25)
		} else {
			t.Fatal("multi-position change did not slide")
		}
		if len(screen.rows) != 24 || len(screen.slides.slots) != 24 {
			t.Fatal("row/animation tracking grew")
		}
	}
	cancelled := screen.slides.animation
	screen.stopSlides()
	if screen.slides.animation != nil {
		t.Fatal("hide did not stop animation")
	}
	for node, slot := range screen.slides.slots {
		if node.Position().Y != slot.target {
			t.Fatal("hide left partial positions")
		}
	}
	cancelled.Tick(0.5)
	for node, slot := range screen.slides.slots {
		if node.Position().Y != slot.target {
			t.Fatal("cancelled hide callback ran")
		}
	}
	model.Rows[0], model.Rows[23] = model.Rows[23], model.Rows[0]
	screen.content(model)
	window.Resize(fyne.NewSize(1400, 920))
	if screen.slides.animation != nil {
		t.Fatal("resize left an animation with obsolete geometry")
	}
	for node, slot := range screen.slides.slots {
		if node.Position().Y != slot.target {
			t.Fatal("resize did not settle rows")
		}
	}
	model.Rows[0], model.Rows[23] = model.Rows[23], model.Rows[0]
	screen.content(model)
	oldLayout := screen.slides
	model.ColumnsOpen = true
	screen.content(model)
	if oldLayout.animation != nil || screen.slides == oldLayout {
		t.Fatal("column panel did not cancel the old layout")
	}
	// New session identities appear directly rather than flying in from y=0.
	for i := range model.Rows {
		model.Rows[i].ID = fmt.Sprintf("next-%d", i)
	}
	screen.slides.start = func(*fyne.Animation) { t.Fatal("new-session rows animated") }
	screen.content(model)
	if len(screen.slides.slots) != 24 || len(screen.rows) != 24 {
		t.Fatal("previous session rows retained")
	}
}

func BenchmarkRetroSlideFrame(b *testing.B) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	model := performanceModel(b)
	screen := &retroScreen{bridge: newDesktopBridge()}
	window := test.NewWindow(screen.content(model))
	defer window.Close()
	window.Resize(fyne.NewSize(1600, 920))
	screen.slides.start = func(*fyne.Animation) {}
	model.Rows[0], model.Rows[23] = model.Rows[23], model.Rows[0]
	screen.content(model)
	animation := screen.slides.animation
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		animation.Tick(float32(i%16) / 16)
	}
}

func TestRetroSlidingToggleDuringAnimation(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	model := performanceModel(t)
	screen := &retroScreen{bridge: newDesktopBridge()}
	root := screen.content(model)
	window := test.NewWindow(root)
	defer window.Close()
	window.Resize(fyne.NewSize(1600, 920))
	starts := 0
	screen.slides.start = func(*fyne.Animation) { starts++ }
	model.Rows[0], model.Rows[1] = model.Rows[1], model.Rows[0]
	screen.content(model)
	old := screen.slides.animation
	if old == nil {
		t.Fatal("default sliding disabled")
	}
	old.Tick(.5)
	model.DisableSlides = true
	if screen.content(model) != root || screen.slides.animation != nil {
		t.Fatal("toggle rebuilt canvas or left animation running")
	}
	for node, slot := range screen.slides.slots {
		if node.Position().Y != slot.target {
			t.Fatal("disable did not settle row")
		}
	}
	old.Tick(.9)
	for node, slot := range screen.slides.slots {
		if node.Position().Y != slot.target {
			t.Fatal("cancelled animation moved row")
		}
	}
	model.Rows[0], model.Rows[1] = model.Rows[1], model.Rows[0]
	screen.content(model)
	if starts != 1 || screen.slides.animation != nil {
		t.Fatal("disabled reorder animated")
	}
	for node, slot := range screen.slides.slots {
		if node.Position().Y != slot.target {
			t.Fatal("disabled row did not jump")
		}
	}
	model.DisableSlides = false
	screen.content(model)
	model.Rows[0], model.Rows[1] = model.Rows[1], model.Rows[0]
	screen.content(model)
	if starts != 2 || screen.slides.animation == nil {
		t.Fatal("re-enabling did not restore sliding")
	}
	screen.stopSlides()
}

func TestClassificationOrderDrivesRetainedSlides(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	a := App{frame: grid(), mode: "full"}
	a.frame.State |= classificationOrder
	screen := &retroScreen{bridge: newDesktopBridge()}
	root := screen.content(a.retroModel(true))
	window := test.NewWindow(root)
	defer window.Close()
	window.Resize(fyne.NewSize(1600, 920))
	starts := 0
	screen.slides.start = func(*fyne.Animation) { starts++ }
	id := rowID(a.frame, a.frame.Cars[0])
	row := screen.rows[id]
	a.frame.Cars[0], a.frame.Cars[1] = a.frame.Cars[1], a.frame.Cars[0]
	screen.content(a.retroModel(true))
	if starts != 1 || screen.slides.animation == nil || screen.rows[id] != row {
		t.Fatal("classification reorder failed retained-row sliding")
	}
	screen.stopSlides()
	a.disableSlides = true
	a.frame.Cars[0], a.frame.Cars[1] = a.frame.Cars[1], a.frame.Cars[0]
	screen.content(a.retroModel(true))
	if starts != 1 || screen.slides.animation != nil || screen.rows[id] != row {
		t.Fatal("classification reorder ignored animation toggle")
	}
}
