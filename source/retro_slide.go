//go:build desktop

package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

const retroSlideDuration = 250 * time.Millisecond

type retroSlideSlot struct {
	target float32
	placed bool
}
type retroRowMotion struct {
	node     fyne.CanvasObject
	from, to float32
}

// Same dimensions as VBox, but registered driver rows can travel to their new
// slots. A single animation moves the whole field; no per-row timers or copies.
// Layout, telemetry and animation callbacks all run on Fyne's UI thread.
type retroSlideLayout struct {
	slots     map[fyne.CanvasObject]*retroSlideSlot
	pending   bool
	enabled   bool
	width     float32
	animation *fyne.Animation
	start     func(*fyne.Animation) // deterministic frame stepping in tests
}

func newRetroSlideLayout() *retroSlideLayout {
	return &retroSlideLayout{enabled: true, slots: map[fyne.CanvasObject]*retroSlideSlot{}}
}
func (l *retroSlideLayout) register(node fyne.CanvasObject) { l.slots[node] = &retroSlideSlot{} }
func (l *retroSlideLayout) stop(settle bool) {
	if l.animation != nil {
		a := l.animation
		l.animation = nil
		a.Stop()
	}
	l.pending = false
	if settle {
		for node, slot := range l.slots {
			if slot.placed {
				node.Move(fyne.NewPos(0, slot.target))
			}
		}
	}
}
func (l *retroSlideLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	count := 0
	for _, child := range objects {
		if !child.Visible() {
			continue
		}
		min := child.MinSize()
		size.Width = fyne.Max(size.Width, min.Width)
		size.Height += min.Height
		count++
	}
	if count > 1 {
		size.Height += float32(count-1) * theme.Padding()
	}
	return size
}
func (l *retroSlideLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	animate := l.enabled && l.pending && l.width == size.Width
	if l.pending || l.width != size.Width {
		l.stop(false)
	}
	l.pending = false
	l.width = size.Width
	var motions []retroRowMotion
	y := float32(0)
	for _, child := range objects {
		if !child.Visible() {
			continue
		}
		height := child.MinSize().Height
		child.Resize(fyne.NewSize(size.Width, height))
		slot := l.slots[child]
		switch {
		case slot == nil:
			child.Move(fyne.NewPos(0, y))
		case animate && slot.placed && child.Position().Y != y:
			motions = append(motions, retroRowMotion{child, child.Position().Y, y})
		case l.animation != nil && slot.placed && slot.target == y:
			// A normal live refresh must not snap or restart an ongoing slide.
		default:
			child.Move(fyne.NewPos(0, y))
		}
		if slot != nil {
			slot.target = y
			slot.placed = true
		}
		y += height + theme.Padding()
	}
	if len(motions) == 0 {
		return
	}
	var animation *fyne.Animation
	animation = fyne.NewAnimation(retroSlideDuration, func(progress float32) {
		if l.animation != animation {
			return
		} // ignore cancelled/replaced callbacks
		for _, motion := range motions {
			motion.node.Move(fyne.NewPos(0, motion.from+(motion.to-motion.from)*progress))
		}
		if progress >= 1 {
			l.animation = nil
		}
	})
	animation.Curve = fyne.AnimationEaseInOut
	l.animation = animation
	if l.start != nil {
		l.start(animation)
	} else {
		animation.Start()
	}
}
