//go:build desktop

package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestRetroColumnDragRetainsLiveObjects(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	a := App{data: t.TempDir(), profileName: "profile", frame: grid(), mode: "full", status: "Live"}
	a.frame.SessionType = 5
	model := a.retroModel(true)
	screen := &retroScreen{bridge: newDesktopBridge()}
	root := screen.content(model)
	window := test.NewWindow(root)
	defer window.Close()
	window.Resize(fyne.NewSize(1600, 920))
	row := screen.rows[model.Rows[0].ID]
	handle := screen.handles["driver"]
	handle.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DX: 60}})
	width := handle.width
	var index int
	for i, col := range screen.columns {
		if col.Key == "driver" {
			index = i
		}
	}
	if screen.columns[index].Width != width || !screen.draggingWidth {
		t.Fatal("drag did not preview")
	}
	model.Rows[0].Cells[0].Text = "99"
	if screen.content(model) != root || screen.rows[model.Rows[0].ID] != row || row.cells[0].text.Text != "99" || screen.columns[index].Width != width {
		t.Fatal("live refresh replaced or reverted active drag")
	}
	handle.DragEnd()
	action := <-screen.bridge.actions
	if action.Kind != "width" || action.Key != "driver" || action.Width != width || action.PreferenceKey != a.preferencePath() {
		t.Fatal("drag was not committed to correct preferences")
	}
	screen.content(model)
	if screen.columns[index].Width != width {
		t.Fatal("old model reverted pending save")
	}
	a.retroAction(action)
	ack := a.retroModel(true)
	if screen.content(ack) != root || screen.pendingWidth || screen.columns[index].Width != width {
		t.Fatal("saved width acknowledgement rebuilt or reverted rows")
	}
	handle.DoubleTapped(nil)
	a.retroAction(<-screen.bridge.actions)
	screen.content(a.retroModel(true))
	if screen.columns[index].Width != 0 || screen.pendingWidth {
		t.Fatal("double-click reset failed")
	}
	handle.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DX: 20}})
	a.frame.SessionType = 15
	screen.content(a.retroModel(true))
	handle.DragEnd()
	select {
	case <-screen.bridge.actions:
		t.Fatal("obsolete layout submitted a drag")
	default:
	}
}
func TestRetroColumnWidthsAndTextClipping(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	cols := []retroColumn{{Key: "driver", Weight: 2, Width: 240}, {Key: "last", Weight: 1}}
	widths := retroColumnWidths(cols, 1050)
	if widths[0] != 240 || widths[1] != 794 {
		t.Fatal("custom width did not remain fixed")
	}
	cols[0].Width = 1200
	min := retroColumnsMinWidth(cols)
	if min < 1248 || retroColumnWidths(cols, min)[1] < 32 {
		t.Fatal("horizontal overflow lost minimum space")
	}
	label := fittedRetroText(retroText("PÉREZ Sergio", retroYellow, 14))
	label.Resize(fyne.NewSize(45, 22))
	if !utf8.ValidString(label.Text.Text) || !strings.HasSuffix(label.Text.Text, "…") {
		t.Fatal("narrow label did not clip safely")
	}
	label.Resize(fyne.NewSize(240, 22))
	if label.Text.Text != "PÉREZ Sergio" {
		t.Fatal("widening did not restore full value")
	}
}

// Hidden columns must not leave a phantom horizontal scrolling area. Exercise
// Fyne's actual scrollbar visibility across overflow and subsequent resizing.
func TestRetroScrollbarsOnlyWhenContentOverflows(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	columns := []retroColumn{{Key: "driver", Weight: 2, Width: 240}, {Key: "last", Weight: 1, Width: 120}}
	row := container.New(retroRowLayout{columns: columns}, fittedRetroText(retroText("NORRIS", retroYellow, 14)), fittedRetroText(retroText("1:20.123", retroWhite, 14)))
	scroll := container.NewScroll(row)
	renderer := test.WidgetRenderer(scroll)
	overlaysVisible := func() bool {
		for _, object := range renderer.Objects()[1:] {
			if object.Visible() {
				return true
			}
		}
		return false
	}
	scroll.Resize(fyne.NewSize(600, 200))
	if overlaysVisible() {
		t.Fatal("scrollbar/shadow visible while every column and row fits")
	}
	scroll.Resize(fyne.NewSize(300, 20))
	if !overlaysVisible() {
		t.Fatal("overflow has no scrollbar")
	}
	scroll.ScrollToOffset(fyne.NewPos(40, 5))
	scroll.Resize(fyne.NewSize(600, 200))
	if overlaysVisible() || scroll.Offset != (fyne.Position{}) {
		t.Fatal("scrollbar or offset remained after content fitted again")
	}
}
