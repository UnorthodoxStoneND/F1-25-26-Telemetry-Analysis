package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Compare the previous unconditional full-page work with the new tick gate.
func BenchmarkLiveRefresh(b *testing.B) {
	for _, scenario := range []string{"stationary", "hidden-setup", "visible-wear"} {
		for _, gated := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/gated=%t", scenario, gated), func(b *testing.B) {
				a := App{frame: grid(), enabled: true, mode: "full", status: "Live"}
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

// A captured v0.5.0 page set verifies that optimisation preserves the complete
// launcher payload, including bar precision, sorting values and editable cells.
func TestViewCompatibility(t *testing.T) {
	path := os.Getenv("TELEMETRY_REFERENCE_VIEWS")
	if path == "" {
		t.Skip("optional comparison with the previous working build")
	}
	var views []any
	for _, mode := range []string{"full", "nearest"} {
		for _, player := range []int32{-1, 0, 12, 23} {
			for _, expanded := range []bool{false, true} {
				a := App{frame: grid(), mode: mode, status: "Live", highlight: true}
				a.frame.Player = player
				for n := range a.frame.Cars {
					c := &a.frame.Cars[n]
					c.Known |= knownLast | knownBest
					c.Last, c.Best = 81000+uint32(n), 80000+uint32(n)
					c.WearKnown = 15
					c.Wear = [4]float32{0, 12.345, 50, 99.99}
					c.SetupKnown = (1 << 21) - 1
					for k := range c.Setup {
						c.Setup[k] = float32(k) + .125
					}
				}
				if expanded {
					a.expanded = rowID(a.frame, a.frame.Cars[12])
				}
				v := a.view()
				v["subtitle"] = "Live · version-normalised"
				views = append(views, v)
			}
		}
	}
	for _, status := range []string{"Paused · values retained", "Session ended", "Add-on disabled", "Live data unavailable"} {
		a := App{frame: Frame{Player: -1, Diagnostic: "Diagnostic"}, mode: "full", status: status}
		v := a.view()
		v["subtitle"] = status + " · version-normalised"
		views = append(views, v)
	}
	data, err := json.Marshal(views)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TELEMETRY_WRITE_REFERENCE") == "1" {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	reference, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var expected, actual any
	if err = json.Unmarshal(reference, &expected); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &actual); err != nil {
		t.Fatal(err)
	}
	// 0.5.2 intentionally changes column sizing; all other legacy fixture
	// contents must still match, including precision and native controls.
	var ignoreColumnWidths func(any)
	ignoreColumnWidths = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if columns, ok := v["columns"].([]any); ok {
				for _, column := range columns {
					if spec, ok := column.(map[string]any); ok {
						delete(spec, "width")
					}
				}
			}
			for _, child := range v {
				ignoreColumnWidths(child)
			}
		case []any:
			for _, child := range v {
				ignoreColumnWidths(child)
			}
		}
	}
	ignoreColumnWidths(expected)
	ignoreColumnWidths(actual)
	if !reflect.DeepEqual(expected, actual) {
		t.Fatal("launcher page JSON differs from the previous working build")
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
		"position":    func(a *App) { a.frame.Cars[0].Position = 2; a.frame.Cars[1].Position = 1 },
		"driver":      func(a *App) { a.frame.Cars[0].Name = "Changed Driver" },
		"name-format": func(a *App) { a.frame.Cars[0].Flags = 6 << 16 },
		"compound":    func(a *App) { a.frame.Cars[0].Compound = 19 },
		"colour":      func(a *App) { a.frame.Cars[0].Visual = 16 },
		"wear":        func(a *App) { a.frame.Cars[0].WearKnown = 15; a.frame.Cars[0].Wear[0] = 12.345 },
		"last":        func(a *App) { a.frame.Cars[0].Known |= knownLast; a.frame.Cars[0].Last = 80000 },
		"best":        func(a *App) { a.frame.Cars[0].Known |= knownBest; a.frame.Cars[0].Best = 79000 },
		"identity":    func(a *App) { a.frame.Cars[0].Identity++ },
		"session":     func(a *App) { a.frame.Session++ },
		"pause":       func(a *App) { a.status = "Paused · values retained" },
		"stale":       func(a *App) { a.status = "No new game values · retained" },
		"diagnostic":  func(a *App) { a.frame.Diagnostic = "Game binding unavailable" },
		"end":         func(a *App) { a.frame = Frame{Player: -1}; a.status = "Session ended" },
		"mode":        func(a *App) { a.mode = "nearest" },
		"highlight":   func(a *App) { a.highlight = true },
		"player":      func(a *App) { a.highlight = true; a.frame.Player = 0 },
		"expand":      func(a *App) { a.expanded = rowID(a.frame, a.frame.Cars[0]) },
		"setup":       func(a *App) { a.frame.Cars[0].SetupKnown = 1 << 20; a.frame.Cars[0].Setup[20] = 55 },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			a := App{frame: grid(), mode: "full", status: "Live"}
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
