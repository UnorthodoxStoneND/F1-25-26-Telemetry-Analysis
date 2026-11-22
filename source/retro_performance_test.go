//go:build desktop

package main

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func performanceModel(t testing.TB) retroModel {
	a := App{frame: grid(), mode: "full", status: "Live", highlight: true}
	a.frame.SessionType = 15
	a.frame.SessionKind = 3
	for i := range a.frame.Cars {
		c := &a.frame.Cars[i]
		c.Known |= knownLast | knownBest | knownResult | knownGapAhead | knownTyreAge | knownSectors
		c.Last = 90000
		c.Best = 89000
		c.Flags = 2 << 8
		c.WearKnown = 15
	}
	return a.retroModel(true)
}

func benchmarkRetroUpdates(b *testing.B, images bool) {
	application := test.NewApp()
	defer application.Quit()
	application.Settings().SetTheme(retroTheme{theme.DefaultTheme()})
	model := performanceModel(b)
	if images {
		frame := badgeFrame()
		var cache badgeCache
		cache.apply(badgeFixture(), &frame)
		for i := range model.Rows {
			for j, col := range model.Columns {
				if col.Key == "team" || col.Key == "tyre" {
					model.Rows[i].Cells[j].PNG = frame.Cars[0].BadgePNG
				}
			}
		}
	}
	screen := &retroScreen{bridge: newDesktopBridge(), images: map[string]fyne.Resource{}}
	window := test.NewWindow(screen.content(model))
	defer window.Close()
	window.Resize(fyne.NewSize(1600, 920))
	b.ReportAllocs()
	b.ResetTimer()
	for tick := 0; tick < b.N; tick++ {
		for row := range model.Rows {
			for col, c := range model.Columns {
				if c.Key == "gap" || c.Key == "FL" || c.Key == "FR" || c.Key == "RL" || c.Key == "RR" || c.Key == "Avg" {
					model.Rows[row].Cells[col].Text = fmt.Sprintf("%d.%03d", row+1, tick%1000)
				}
			}
		}
		content := screen.content(model)
		if content != window.Content() {
			window.SetContent(content)
		}
	}
}

func BenchmarkRetroLiveUpdates(b *testing.B) {
	benchmarkRetroUpdates(b, false)
}

func BenchmarkRetroLiveUpdatesWithImages(b *testing.B) { benchmarkRetroUpdates(b, true) }
