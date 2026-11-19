package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

// Compare the previous unconditional full-page work with the new tick gate.
func BenchmarkLiveRefresh(b *testing.B) {
	for _, scenario := range []string{"stationary", "hidden-setup", "visible-wear"} {
		for _, gated := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/gated=%t", scenario, gated), func(b *testing.B) {
				a := App{frame: grid(), enabled: true, mode: "full", status: "Live"}
				if scenario == "visible-wear" {
					a.frame.SessionType = 15
				}
				a.lastView, a.viewSent = a.viewState(), true
				b.ReportAllocs()
				b.ResetTimer()
				for n := 0; n < b.N; n++ {
					a.frame.Tick++
					if scenario == "hidden-setup" {
						a.frame.Cars[0].SetupKnown = 1 << 20
						a.frame.Cars[0].Setup[20] = float32(n % 100)
					}
					if scenario == "visible-wear" {
						a.frame.Cars[0].WearKnown = 15
						a.frame.Cars[0].Wear[0] = float32(n % 100)
					}
					if gated && !a.needsView() {
						continue
					}
					if _, err := json.Marshal(a.view()); err != nil {
						b.Fatal(err)
					}
					if gated {
						a.lastView = a.viewState()
					}
				}
			})
		}
	}
}

func TestReusedSnapshotStorage(t *testing.T) {
	var storage [maxCars]Car
	b := fixture()
	first, err := decodeInto(b, storage[:0])
	if err != nil {
		t.Fatal(err)
	}
	if &first.Cars[0] != &storage[0] {
		t.Fatal("snapshot storage was not reused")
	}
	// Shrinking and regrowing snapshots must never retain a removed driver's data.
	b[48] = 0
	empty, err := decodeInto(b, storage[:0])
	if err != nil || len(empty.Cars) != 0 {
		t.Fatal("empty snapshot retained drivers")
	}
	b = fixture()
	b[256+32] = 1
	second, err := decodeInto(b, storage[:0])
	if err != nil || len(second.Cars) != 1 || second.Cars[0].WearKnown != 1 {
		t.Fatal("reused snapshot retained old fields")
	}
}

func TestRefreshPreservesVisibleChanges(t *testing.T) {
	changes := map[string]func(*App){
		"position":   func(a *App) { a.frame.Cars[0].Position = 2; a.frame.Cars[1].Position = 1 },
		"driver":     func(a *App) { a.frame.Cars[0].Name = "Changed Driver" },
		"compound":   func(a *App) { a.frame.Cars[0].Compound = 19 },
		"tyre-image": func(a *App) { a.frame.Cars[0].TyrePNG = testBadgePNG(t) },
		"wear":       func(a *App) { a.frame.Cars[0].WearKnown = 15; a.frame.Cars[0].Wear[0] = 12.345 },
		"last":       func(a *App) { a.frame.Cars[0].Known |= knownLast; a.frame.Cars[0].Last = 80000 },
		"best":       func(a *App) { a.frame.Cars[0].Known |= knownBest; a.frame.Cars[0].Best = 79000 },
		"identity":   func(a *App) { a.frame.Cars[0].Identity++ },
		"session":    func(a *App) { a.frame.Session++ },
		"pause":      func(a *App) { a.status = "Paused · values retained" },
		"stale":      func(a *App) { a.status = "No new game values · retained" },
		"diagnostic": func(a *App) { a.frame.Diagnostic = "Game binding unavailable" },
		"end":        func(a *App) { a.frame = Frame{Player: -1}; a.status = "Session ended" },
		"mode":       func(a *App) { a.mode = "nearest" },
		"highlight":  func(a *App) { a.highlight = true },
		"player":     func(a *App) { a.highlight = true; a.frame.Player = 0 },
		"expand":     func(a *App) { a.expanded = rowID(a.frame, a.frame.Cars[0]) },
		"setup":      func(a *App) { a.frame.Cars[0].SetupKnown = 1 << 20; a.frame.Cars[0].Setup[20] = 55 },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			a := App{frame: grid(), mode: "full", status: "Live"}
			if name == "compound" {
				a.frame.Cars[0].TyrePNG = testBadgePNG(t)
			}
			if name == "wear" {
				a.frame.SessionType = 15
			}
			if name == "setup" {
				a.expanded = rowID(a.frame, a.frame.Cars[0])
			}
			before, _ := json.Marshal(a.view())
			a.lastView, a.viewSent = a.viewState(), true
			change(&a)
			after, _ := json.Marshal(a.view())
			if bytes.Equal(before, after) {
				t.Fatal("fixture did not change the visible page")
			}
			if !a.needsView() {
				t.Fatal("visible change would not reach the launcher")
			}
		})
	}
}

func TestUnchangedTicksDoNotInvalidate(t *testing.T) {
	a := App{frame: grid(), mode: "full", status: "Live"}
	a.lastView, a.viewSent = a.viewState(), true
	for n := 0; n < 10000; n++ {
		a.frame.Tick++
		a.frame.Cars[0].SetupKnown = 1 << 20
		a.frame.Cars[0].Setup[20] = float32(n % 100)
		// Unknown data, including NaN, must not cause repeated page rebuilds.
		a.frame.Cars[0].Wear[0] = float32(math.NaN())
		if a.needsView() {
			t.Fatal("non-displayed sample data invalidated the page")
		}
	}
	// Exercise the actual protocol tick path while disabled; no game required.
	input := `{"id":1,"method":"hello","params":{"api":1,"data_dir":"unused","profile":{"enabled":false}}}` + "\n" +
		`{"id":2,"method":"view","params":{"section":"telemetry"}}` + "\n"
	for n := 0; n < 100; n++ {
		input += fmt.Sprintf("{\"id\":%d,\"method\":\"tick\",\"params\":{\"section\":\"telemetry\"}}\n", n+3)
	}
	var output bytes.Buffer
	if err := run(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output.Bytes(), []byte(`"invalidate"`)) {
		t.Fatal("idle protocol ticks invalidated the launcher")
	}
	if lines := bytes.Count(output.Bytes(), []byte("\n")); lines != 102 {
		t.Fatalf("missing tick replies: %d", lines)
	}
}

func TestNearestRefreshIgnoresHiddenCars(t *testing.T) {
	a := App{frame: grid(), mode: "nearest", status: "Live"}
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.Cars[0].Known |= knownLast
	a.frame.Cars[0].Last = 80000
	if a.needsView() {
		t.Fatal("off-screen lap invalidated nearest display")
	}
	a.frame.Cars[0].Position = 12
	if !a.needsView() {
		t.Fatal("car entering the nearest window was missed")
	}
}
