//go:build desktop

package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestRetroRetainedRowsAndImages(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	model := performanceModel(t)
	png := testBadgePNG(t)
	for i := range model.Rows {
		for j, col := range model.Columns {
			if col.Key == "team" || col.Key == "tyre" {
				model.Rows[i].Cells[j].PNG = png
			}
		}
	}
	screen := &retroScreen{bridge: newDesktopBridge()}
	root := screen.content(model)
	window := test.NewWindow(root)
	defer window.Close()
	window.Resize(fyne.NewSize(1600, 920))
	first := screen.rows[model.Rows[0].ID]
	var team, last, position int
	for i, col := range model.Columns {
		switch col.Key {
		case "team":
			team = i
		case "last":
			last = i
		case "position":
			position = i
		}
	}
	image := first.cells[team].image
	resource := image.Resource
	label := first.cells[last].text
	for tick := 0; tick < 2000; tick++ {
		model.Rows[0].Cells[last].Text = fmt.Sprintf("1:30.%03d", tick%1000)
		if screen.content(model) != root || screen.rows[model.Rows[0].ID] != first || first.cells[team].image != image || image.Resource != resource || first.cells[last].text != label {
			t.Fatal("ordinary updates replaced canvas objects")
		}
	}
	if len(screen.rows) != 24 || len(screen.images) != 1 {
		t.Fatal("live caches grew with samples")
	}
	model.Rows[0], model.Rows[1] = model.Rows[1], model.Rows[0]
	if screen.content(model) != root || screen.order[1] != model.Rows[1].ID || screen.rows[model.Rows[1].ID] != first {
		t.Fatal("position change recreated row")
	}
	model.Rows[1].Retired = true
	model.Rows[1].Player = true
	screen.content(model)
	if image.Translucency != 0.55 || label.Color != retroMuted || first.background.FillColor != retroBlue || !strings.HasPrefix(first.cells[position].text.Text, "▸") {
		t.Fatal("retirement/highlight update missing")
	}
	model.Rows[1].Cells[team].PNG = ""
	screen.content(model)
	if image.Visible() || image.Resource != nil || !first.cells[team].text.Visible() {
		t.Fatal("unavailable image left stale badge")
	}
	model.Rows[1].Cells[team].PNG = png
	model.Rows[1].Retired = false
	screen.content(model)
	if !image.Visible() || image.Resource != resource || image.Translucency != 0 {
		t.Fatal("image did not recover in place")
	}
	removed := model.Rows[1].ID
	model.Rows = append(model.Rows[:1], model.Rows[2:]...)
	screen.content(model)
	if len(screen.rows) != 23 || screen.rows[removed] != nil {
		t.Fatal("departed car retained in row cache")
	}
}

func TestRetroRetainedControlsAndLayoutTransitions(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	a := App{frame: grid(), mode: "full", status: "Live", columnsOpen: true, editorLayout: layoutPractice}
	a.frame.SessionType = 1
	a.expanded = rowID(a.frame, a.frame.Cars[0])
	screen := &retroScreen{bridge: newDesktopBridge()}
	model := a.retroModel(true)
	root := screen.content(model)
	setup := screen.setupValues[0]
	check := screen.checks["last"]
	model.Status = "Paused · values retained"
	model.Setup[0] = "Front wing  12"
	for i := range model.Choices {
		if model.Choices[i].Key == "last" {
			model.Choices[i].Visible = false
		}
	}
	if screen.content(model) != root || screen.setupValues[0] != setup || setup.Text != "Front wing  12" || screen.state.Text != "PAUSED" || screen.checks["last"] != check || check.Checked {
		t.Fatal("live setup/column/pause controls were replaced or stale")
	}
	select {
	case <-screen.bridge.actions:
		t.Fatal("model update submitted a user preference change")
	default:
	}
	check.SetChecked(true)
	action := <-screen.bridge.actions
	if action.Kind != "field" || action.Key != "last" || !action.Visible || action.Layout != layoutPractice {
		t.Fatal("column callback lost")
	}
	id := model.Rows[0].ID
	for i, col := range model.Columns {
		if col.Key == "setup" {
			screen.rows[id].cells[i].node.(*retroTap).Tapped(nil)
		}
	}
	if action := <-screen.bridge.actions; action.Kind != "setup" || action.Row != id {
		t.Fatal("setup callback lost")
	}
	model.Columns = model.Columns[:len(model.Columns)-1]
	for i := range model.Rows {
		model.Rows[i].Cells = model.Rows[i].Cells[:len(model.Columns)]
	}
	if screen.content(model) == root {
		t.Fatal("column change did not rebuild layout")
	}
	allHidden := retroModel{Show: true, Message: "All columns are hidden", Layout: layoutPractice, ColumnsOpen: true, Choices: model.Choices}
	screen.content(allHidden)
	if len(screen.rows) != 0 || len(screen.images) != 0 || screen.message.Text != allHidden.Message {
		t.Fatal("all hidden retained data objects")
	}
	for _, typ := range []uint32{1, 5, 15} {
		a.frame.SessionType = typ
		a.expanded = ""
		a.columnsOpen = false
		expected := a.retroModel(true)
		screen.content(expected)
		if len(screen.columns) != len(expected.Columns) || len(screen.rows) != 24 || !reflect.DeepEqual(screen.order, rowIDs(expected)) {
			t.Fatal("session layout transition lost fields")
		}
	}
}

func rowIDs(m retroModel) []string {
	ids := []string{}
	for _, row := range m.Rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestRetroRenderedViews(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	for _, typ := range []uint32{1, 5, 15} {
		a := App{frame: grid(), mode: "full", highlight: true, status: "Live"}
		a.frame.SessionType = typ
		if typ == 5 {
			a.frame.Player = 22
		}
		a.frame.SessionKind = 3
		a.frame.State |= sessionTimer
		a.frame.Progress = 750
		if typ == 15 {
			a.frame.State = active | sessionLaps
			a.frame.Progress = 57<<16 | 8
		}
		for i := range a.frame.Cars {
			c := &a.frame.Cars[i]
			c.Known |= knownLast | knownBest | knownResult | knownPit | knownLaps | knownTyreAge | knownSectors | knownLastSectors | knownGapAhead | knownPenalties | knownTimePenalty
			c.BadgePNG = testBadgePNG(t)
			c.TyrePNG = testBadgePNG(t)
			c.Last = 89215 + uint32(i)*233
			c.Best = 88902 + uint32(i)*299
			c.Laps = 8
			c.TyreAgeLaps = 3
			c.BestSectors = [3]uint32{29030, 39120, 21065}
			c.LastSectors = [3]uint32{30000, 39000, c.Last - 69000}
			c.GapAhead = int32(i * 843)
			if i == 2 || i == 4 {
				c.TimePenaltyMS = 5000
			}
			if i == 4 {
				c.DriveThrough = 1
			}
			if i == 7 {
				c.StopGo = 1
			}
			if i == 8 {
				c.DriveThrough, c.StopGo = 2, 1
			}
			c.Flags = 2 << 8
			c.WearKnown = 15
			c.Wear = [4]float32{20, 23, 19, 22}
		}
		a.frame.Cars[3].Flags = inPitLane | 2<<8
		a.frame.Cars[23].Flags = 4 << 8
		m := a.retroModel(true)
		screen := retroScreen{bridge: newDesktopBridge(), images: map[string]fyne.Resource{}}
		window := test.NewWindow(screen.content(m))
		window.Resize(fyne.NewSize(1600, 920))
		picture := window.Canvas().Capture()
		if picture.Bounds().Dx() < 1000 {
			t.Fatal("window layout unexpectedly collapsed")
		}
		if dir := os.Getenv("RETRO_SCREENSHOTS"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			file, err := os.Create(filepath.Join(dir, layoutKeys[layoutFor(a.frame)]+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err = png.Encode(file, picture); err != nil {
				t.Fatal(err)
			}
			file.Close()
		}
		window.Close()
	}
}

func TestRetroRiskColourChangesInPlace(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	a := App{frame: riskGrid(24), mode: "full", highlight: true}
	a.frame.SessionType = 5
	a.frame.Player = 18
	screen := &retroScreen{bridge: newDesktopBridge()}
	root := screen.content(a.retroModel(true))
	window := test.NewWindow(root)
	defer window.Close()
	window.Resize(fyne.NewSize(1600, 920))
	id := rowID(a.frame, a.frame.Cars[18])
	row := screen.rows[id]
	if row.background.FillColor != retroBlue {
		t.Fatal("at-risk player lost blue highlight")
	}
	other := screen.rows[rowID(a.frame, a.frame.Cars[17])]
	if other.background.FillColor != retroEliminationRed {
		t.Fatal("other at-risk driver lost red")
	}
	a.highlight = false
	screen.content(a.retroModel(true))
	if row.background.FillColor != retroEliminationRed || screen.rows[id] != row {
		t.Fatal("disabled player highlight did not restore red in place")
	}
	a.highlight = true
	screen.content(a.retroModel(true))
	if row.background.FillColor != retroBlue || screen.rows[id] != row {
		t.Fatal("re-enabled player highlight did not restore blue in place")
	}
	a.frame.Cars[18].Position = 17
	screen.content(a.retroModel(true))
	if screen.rows[id] != row || row.background.FillColor != retroBlue {
		t.Fatal("safe row recreated or failed to restore player colour")
	}
	a.frame.SessionType = 7
	screen.content(a.retroModel(true))
	if screen.rows[id].background.FillColor == retroEliminationRed {
		t.Fatal("Q3 retained red background")
	}
}
