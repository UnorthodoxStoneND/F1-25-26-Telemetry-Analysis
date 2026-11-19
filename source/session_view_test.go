package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func rowCells(t *testing.T, a *App, vehicle uint32) map[string]any {
	t.Helper()
	target := ""
	for _, c := range a.frame.Cars {
		if c.Vehicle == vehicle {
			target = rowID(a.frame, c)
		}
	}
	for _, block := range a.view()["body"].(M)["blocks"].([]M) {
		table, ok := block["table"].(M)
		if !ok {
			continue
		}
		cols := table["columns"].([]M)
		for _, row := range table["rows"].([]M) {
			cells := row["cells"].([]any)
			if len(cells) != len(cols) {
				t.Fatalf("column/cell mismatch: %d/%d", len(cols), len(cells))
			}
			if row["id"] != target {
				continue
			}
			result := map[string]any{}
			for i, col := range cols {
				result[col["key"].(string)] = cells[i]
			}
			return result
		}
	}
	t.Fatalf("vehicle %d missing", vehicle)
	return nil
}
func cellText(t *testing.T, v any) string {
	t.Helper()
	if value, ok := v.(string); ok {
		return value
	}
	if value, ok := v.(uint32); ok {
		return fmt.Sprint(value)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var value struct{ Text string }
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value.Text
}
func TestSessionSpecificColumnsAndBadges(t *testing.T) {
	practice := []string{"position", "team", "driver", "tyre", "laps", "last", "best", "status", "setup"}
	qualifying := []string{"position", "team", "driver", "tyre", "laps", "gap", "last", "best", "status", "setup"}
	racing := []string{"position", "gain", "team", "driver", "tyre", "stint", "gap", "last", "best", "S1", "S2", "S3", "FL", "FR", "RL", "RR", "Avg", "status"}
	for id := uint32(1); id <= 18; id++ {
		a := App{frame: grid(), mode: "full"}
		a.frame.SessionType = id
		a.frame.Cars[0].BadgePNG = testBadgePNG(t)
		expected := practice
		if id >= 5 && id <= 14 {
			expected = qualifying
		}
		if id >= 15 && id <= 17 {
			expected = racing
		}
		got := []string{}
		for _, col := range columns(layoutFor(a.frame)) {
			got = append(got, col["key"].(string))
		}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("session %d: %v", id, got)
		}
		cells := rowCells(t, &a, 0)
		if _, ok := cells["team"].(M)["image_data"]; !ok {
			t.Fatalf("session %d lost team badge", id)
		}
		if _, ok := cells["laps_on_tyre"]; ok {
			t.Fatal("tyre age must be absent")
		}
	}
	for kind, expected := range map[uint32]sessionLayout{0: layoutPractice, 1: layoutPractice, 2: layoutQualifying, 3: layoutRace, 4: layoutRace, 5: layoutPractice} {
		if got := layoutFor(Frame{SessionKind: kind}); got != expected {
			t.Fatal("context-only layout", kind, got)
		}
	}
}
func TestQualifyingGapUsesEntireSessionBest(t *testing.T) {
	a := App{frame: grid(), mode: "nearest"}
	a.frame.SessionType = 10
	for i := range a.frame.Cars {
		a.frame.Cars[i].Known |= knownBest
		a.frame.Cars[i].Best = 80100
	}
	a.frame.Cars[0].Best = 78000
	a.frame.Cars[0].Known |= knownResult
	a.frame.Cars[0].Flags = 5 << 8
	a.frame.Cars[23].Best = 79000
	cells := rowCells(t, &a, 10)
	if got := cellText(t, cells["gap"]); got != "+1.100" {
		t.Fatal("gap is not to eligible session best", got)
	}
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.Cars[23].Best = 78500
	if !a.needsView() {
		t.Fatal("off-screen session best did not refresh visible gaps")
	}
	if got := cellText(t, rowCells(t, &a, 10)["gap"]); got != "+1.600" {
		t.Fatal(got)
	}
}
func TestRaceValuesAndUnknowns(t *testing.T) {
	a := App{frame: grid(), mode: "full"}
	a.frame.SessionType = 15
	c := &a.frame.Cars[0]
	c.Position = 2
	a.frame.Cars[1].Position = 1
	c.Grid = 5
	c.GapAhead = 1234
	c.Best = 80000
	c.BestSectors = [3]uint32{20000, 30000, 30000}
	c.Known |= knownGrid | knownGapAhead | knownBest | knownSectors | knownResult
	c.Flags = 2 << 8
	c.WearKnown = 15
	c.Wear = [4]float32{10, 20, 30, 40}
	cells := rowCells(t, &a, 0)
	for key, want := range map[string]string{"gain": "+3", "gap": "+1.234", "S1": "20.000", "S2": "30.000", "S3": "30.000", "FL": "30.0%", "FR": "40.0%", "RL": "10.0%", "RR": "20.0%", "Avg": "25.0%", "status": "Run"} {
		if got := cellText(t, cells[key]); got != want {
			t.Fatalf("%s: %q want %q", key, got, want)
		}
	}
	c.GapAhead = 0
	if got := cellText(t, rowCells(t, &a, 0)["gap"]); got != "+0.000" {
		t.Fatal("valid zero interval lost", got)
	}
	c.Known &^= knownGrid | knownGapAhead | knownSectors
	c.WearKnown = 7
	cells = rowCells(t, &a, 0)
	for _, key := range []string{"gain", "gap", "S1", "S2", "S3", "Avg"} {
		if cellText(t, cells[key]) != "—" {
			t.Fatal("unknown value fabricated", key)
		}
	}
}
func TestLapsAndSessionOnlyRefresh(t *testing.T) {
	a := App{frame: grid(), mode: "full"}
	a.frame.SessionType = 1
	c := &a.frame.Cars[0]
	c.Known |= knownLaps
	if got := cellText(t, rowCells(t, &a, 0)["laps"]); got != "0" {
		t.Fatal("known zero laps lost", got)
	}
	a.lastView, a.viewSent = a.viewState(), true
	c.WearKnown = 15
	c.Wear = [4]float32{10, 20, 30, 40}
	c.Known |= knownSectors | knownGapAhead
	c.GapAhead = 1234
	if a.needsView() {
		t.Fatal("hidden racing values refreshed practice")
	}
	c.Laps = 7
	if !a.needsView() {
		t.Fatal("laps did not refresh practice")
	}
	a.frame.SessionType = 15
	a.lastView = a.viewState()
	c.Laps = 8
	c.SetupKnown = 1
	c.Setup[0] = 50
	if a.needsView() {
		t.Fatal("hidden laps/setup refreshed racing")
	}
}
func TestRaceHasNoSetupAndTimedViewsDo(t *testing.T) {
	a := App{frame: grid(), mode: "full"}
	a.expanded = rowID(a.frame, a.frame.Cars[0])
	a.frame.SessionType = 15
	for _, block := range a.view()["body"].(M)["blocks"].([]M) {
		if block["card"] != nil {
			t.Fatal("race retained setup panel")
		}
	}
	action, err := a.event(json.RawMessage(fmt.Sprintf("{\"Event\":{\"Submit\":{\"Form\":\"drivers\",\"Row\":%q,\"Values\":{\"setup\":true}}}}", a.expanded)))
	if err != nil || action != "none" {
		t.Fatal("race accepted stale setup action", action, err)
	}
	for _, id := range []uint32{1, 5, 10} {
		a.frame.SessionType = id
		cells := rowCells(t, &a, 0)
		if _, ok := cells["setup"]; !ok {
			t.Fatal("timed session lost setup")
		}
		found := false
		for _, block := range a.view()["body"].(M)["blocks"].([]M) {
			found = found || block["card"] != nil
		}
		if !found {
			t.Fatal("setup panel missing", id)
		}
	}
}
func TestExtendedSnapshotValidation(t *testing.T) {
	b := fixture()
	at := 256
	binary.LittleEndian.PutUint32(b[at+4:], knownBest|knownSectors|knownLaps|knownGapAhead)
	binary.LittleEndian.PutUint32(b[at+28:], 80000)
	binary.LittleEndian.PutUint32(b[at+280:], 7)
	binary.LittleEndian.PutUint32(b[at+284:], 1234)
	for i, ms := range []uint32{20000, 30000, 30000} {
		binary.LittleEndian.PutUint32(b[at+288+4*i:], ms)
	}
	f, err := decode(b)
	if err != nil || f.Cars[0].Known&knownSectors == 0 || f.Cars[0].Laps != 7 || f.Cars[0].GapAhead != 1234 {
		t.Fatal("extended ABI invalid", err)
	}
	binary.LittleEndian.PutUint32(b[at+288:], 10000)
	binary.LittleEndian.PutUint32(b[at+280:], 1000)
	binary.LittleEndian.PutUint32(b[at+284:], 0xffffffff)
	f, err = decode(b)
	if err != nil || f.Cars[0].Known&(knownSectors|knownLaps|knownGapAhead) != 0 {
		t.Fatal("malformed timing/progress fields accepted", err)
	}
}
func TestNoCombinedNameStatusOrWearBars(t *testing.T) {
	a := App{frame: grid(), mode: "full"}
	a.frame.SessionType = 15
	a.frame.Cars[0].Known |= knownPit
	a.frame.Cars[0].Flags |= inPitLane
	a.frame.Cars[0].WearKnown = 15
	cells := rowCells(t, &a, 0)
	if strings.Contains(cellText(t, cells["driver"]), "Pit") || cellText(t, cells["status"]) != "Pit" {
		t.Fatal("status not separated from driver")
	}
	raw, _ := json.Marshal(cells["FL"])
	if strings.Contains(string(raw), "\"bar\"") {
		t.Fatal("race mockup uses compact percentages")
	}
}

// Swofty 1.9.76 Cell is an untagged union of String and CellSpec.
// Exercise populated values: missing readings alone hide bare-number regressions.
func TestSessionCellWireFormats(t *testing.T) {
	for _, session := range []uint32{1, 5, 10, 15, 16} {
		for _, laps := range []uint32{0, 7} {
			a := App{frame: grid(), mode: "full"}
			a.frame.SessionType = session
			c := &a.frame.Cars[0]
			c.Known |= knownLaps | knownGrid | knownGapAhead | knownBest | knownSectors | knownResult
			c.Laps = laps
			c.Grid = 3
			c.GapAhead = 1234
			c.Best = 80000
			c.BestSectors = [3]uint32{20000, 30000, 30000}
			c.Flags = 2 << 8
			c.WearKnown = 15
			raw, err := json.Marshal(a.view())
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]any
			if err = json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			for _, block := range wire["body"].(map[string]any)["blocks"].([]any) {
				table, ok := block.(map[string]any)["table"].(map[string]any)
				if !ok {
					continue
				}
				for _, row := range table["rows"].([]any) {
					for _, cell := range row.(map[string]any)["cells"].([]any) {
						switch v := cell.(type) {
						case string:
						case map[string]any:
							if text, exists := v["text"]; exists {
								if _, ok := text.(string); !ok {
									t.Fatalf("non-text CellSpec: %s", raw)
								}
							}
						default:
							t.Fatalf("session %d: unsupported Swofty cell %T (%v)", session, cell, cell)
						}
					}
				}
			}
			if session < 15 {
				cell := rowCells(t, &a, 0)["laps"]
				if cellText(t, cell) != fmt.Sprint(laps) {
					t.Fatal("lap display lost", cell)
				}
				if cell.(M)["sort"] != laps {
					t.Fatal("lap numeric sort lost", cell)
				}
			}
		}
	}
}
