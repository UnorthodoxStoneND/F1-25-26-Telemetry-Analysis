//go:build desktop

package main

import (
	"image/color"
	"math"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

func sameRetroColumns(a, b []retroColumn) bool {
	if len(a) != len(b) {
		return false
	}
	for i, c := range a {
		if c.Key != b[i].Key || c.Title != b[i].Title || c.Weight != b[i].Weight {
			return false
		}
	}
	return true
}
func retroColumnWidths(columns []retroColumn, width float32) []float32 {
	out := make([]float32, len(columns))
	fixed, weight := float32(0), float32(0)
	for _, col := range columns {
		if col.Width > 0 {
			fixed += col.Width
		} else {
			weight += col.Weight
		}
	}
	remaining := width - 16 - fixed
	for i, col := range columns {
		if col.Width > 0 {
			out[i] = col.Width
		} else if weight > 0 {
			out[i] = remaining * col.Weight / weight
		}
	}
	return out
}
func retroColumnsMinWidth(columns []retroColumn) float32 {
	sum := float32(16)
	for _, col := range columns {
		if col.Width > 0 {
			sum += col.Width
		} else {
			sum += 32
		}
	}
	return sum
}

// Keep the original text for future widening, rendering a bounded monospaced
// label so resized cells cannot draw over their neighbours.
type retroFittedText struct {
	widget.BaseWidget
	Text                *canvas.Text
	full                string
	charWidth, fontSize float32
}

func fittedRetroText(text *canvas.Text) *retroFittedText {
	t := &retroFittedText{Text: text, full: text.Text}
	t.ExtendBaseWidget(t)
	return t
}
func (t *retroFittedText) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.Text)
}
func (t *retroFittedText) MinSize() fyne.Size    { return fyne.NewSize(0, t.Text.MinSize().Height) }
func (t *retroFittedText) Resize(size fyne.Size) { t.BaseWidget.Resize(size); t.fit() }
func (t *retroFittedText) set(value string, colour color.Color) {
	changed := t.full != value || t.Text.Color != colour
	t.full = value
	t.Text.Color = colour
	if changed {
		t.fit()
		t.Text.Refresh()
	}
}
func (t *retroFittedText) fit() {
	if t.fontSize != t.Text.TextSize || t.charWidth == 0 {
		t.fontSize = t.Text.TextSize
		t.charWidth = fyne.MeasureText("M", t.Text.TextSize, t.Text.TextStyle).Width
	}
	count := int(math.Floor(float64(t.Size().Width / t.charWidth)))
	value := t.full
	if count <= 0 {
		value = ""
	} else if utf8.RuneCountInString(value) > count {
		runes := []rune(value)
		value = string(runes[:count-1]) + "…"
	}
	if t.Text.Text != value {
		t.Text.Text = value
		t.Text.Refresh()
	}
}

type retroColumnHandle struct {
	widget.BaseWidget
	start    func() float32
	preview  func(float32)
	commit   func(float32)
	reset    func()
	dragging bool
	width    float32
}

func newRetroColumnHandle() *retroColumnHandle {
	h := &retroColumnHandle{}
	h.ExtendBaseWidget(h)
	return h
}
func (h *retroColumnHandle) Cursor() desktop.Cursor { return desktop.HResizeCursor }
func (h *retroColumnHandle) Dragged(event *fyne.DragEvent) {
	if !h.dragging {
		h.width = h.start()
		h.dragging = true
	}
	h.width = max(32, min(1200, h.width+event.Dragged.DX))
	h.preview(h.width)
}
func (h *retroColumnHandle) DragEnd() {
	if h.dragging {
		h.dragging = false
		h.commit(h.width)
	}
}
func (h *retroColumnHandle) DoubleTapped(*fyne.PointEvent) { h.reset() }
func (h *retroColumnHandle) CreateRenderer() fyne.WidgetRenderer {
	return &retroHandleRenderer{line: canvas.NewLine(retroMuted)}
}

type retroHandleRenderer struct{ line *canvas.Line }

func (r *retroHandleRenderer) Layout(size fyne.Size) {
	r.line.Position1 = fyne.NewPos(size.Width/2, 2)
	r.line.Position2 = fyne.NewPos(size.Width/2, size.Height-2)
}
func (r *retroHandleRenderer) MinSize() fyne.Size           { return fyne.NewSize(8, 22) }
func (r *retroHandleRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.line} }
func (r *retroHandleRenderer) Refresh()                     { r.line.Refresh() }
func (r *retroHandleRenderer) Destroy()                     {}

func (s *retroScreen) columnHeading(index int) fyne.CanvasObject {
	col := s.columns[index]
	handle := newRetroColumnHandle()
	s.handles[col.Key] = handle
	epoch := s.widthGeneration
	var cell *fyne.Container
	handle.start = func() float32 {
		if epoch == s.widthGeneration {
			s.draggingWidth = true
		}
		return cell.Size().Width + 5
	}
	handle.preview = func(width float32) {
		if epoch != s.widthGeneration {
			return
		}
		s.columns[index].Width = width
		s.body.Refresh()
	}
	send := func(width float32) {
		if epoch != s.widthGeneration {
			return
		}
		s.draggingWidth = false
		s.pendingWidth = true
		s.pendingKey = col.Key
		s.pendingValue = width
		s.columns[index].Width = width
		s.body.Refresh()
		s.bridge.act(desktopAction{Kind: "width", Key: col.Key, Width: width, Layout: s.layout, PreferenceKey: s.preferenceKey})
	}
	handle.commit = send
	handle.reset = func() { send(0) }
	cell = containerColumnHeading(col.Title, handle)
	return cell
}
func containerColumnHeading(title string, handle *retroColumnHandle) *fyne.Container {
	return container.New(retroHeadingLayout{}, fittedRetroText(retroText(strings.ToUpper(title), retroYellow, 12)), handle)
}

// Avoid theme padding consuming the narrow position and badge headings.
type retroHeadingLayout struct{}

func (retroHeadingLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 22) }
func (retroHeadingLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(max(0, size.Width-8), size.Height))
	objects[1].Move(fyne.NewPos(max(0, size.Width-8), 0))
	objects[1].Resize(fyne.NewSize(8, size.Height))
}
