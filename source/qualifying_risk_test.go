package main

import (
	"fmt"
	"testing"
)

func riskGrid(n int) Frame {
	f := grid()
	f.Cars = f.Cars[:0]
	for i := 0; i < n; i++ {
		f.Cars = append(f.Cars, Car{Vehicle: uint32(i), Identity: uint64(i + 1), Known: knownName | knownPosition, Position: int32(i + 1), Name: fmt.Sprint(i + 1)})
	}
	return f
}
func TestCalculatedQualifyingZones(t *testing.T) {
	for _, n := range []int{10, 20, 21, 22, 24, 26, 32} {
		f := riskGrid(n)
		for _, session := range []uint32{1, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15} {
			f.SessionType = session
			cutoff := 0
			if n > 10 {
				switch session {
				case 5, 10:
					cutoff = n - (n-10)/2
				case 6, 11:
					cutoff = 10
				}
			}
			for _, c := range f.Cars {
				want := cutoff > 0 && int(c.Position) > cutoff
				if qualifyingAtRisk(f, c) != want {
					t.Fatalf("field %d session %d position %d", n, session, c.Position)
				}
			}
		}
	}
	f := riskGrid(24)
	f.SessionType = 5
	c := f.Cars[23]
	c.Known &^= knownPosition
	if qualifyingAtRisk(f, c) {
		t.Fatal("unknown position marked")
	}
	c.Known |= knownPosition | knownResult
	for _, status := range []uint32{4, 5, 6, 7} {
		c.Flags = status << 8
		if qualifyingAtRisk(f, c) {
			t.Fatal("already out marked at risk", status)
		}
	}
}
func TestQualifyingRiskSharedViewsAndHiddenPosition(t *testing.T) {
	a := App{frame: riskGrid(24), mode: "nearest", highlight: true}
	a.frame.SessionType = 5
	a.frame.Player = 18
	// Hide the position column: risk colour must still refresh as cars cross.
	a.hidden[layoutQualifying] = fieldBit(layoutQualifying, "position")
	tone := func(id string) any {
		for _, block := range a.view()["body"].(M)["blocks"].([]M) {
			if table, ok := block["table"].(M); ok {
				for _, row := range table["rows"].([]M) {
					if row["id"] == id {
						return row["tone"]
					}
				}
			}
		}
		t.Fatal("row missing", id)
		return nil
	}
	c := a.frame.Cars[18]
	if tone(rowID(a.frame, c)) != "accent" {
		t.Fatal("elimination zone masked player highlight")
	}
	if tone(rowID(a.frame, a.frame.Cars[17])) != "bad" {
		t.Fatal("other at-risk driver lost red highlight")
	}
	a.highlight = false
	if tone(rowID(a.frame, c)) != "bad" {
		t.Fatal("disabled player highlight suppressed elimination red")
	}
	a.highlight = true
	m := a.retroModel(true)
	found := false
	for _, row := range m.Rows {
		if row.ID == rowID(a.frame, c) {
			found = true
			if !row.Player || !row.AtRisk {
				t.Fatal("retro risk/player identity missing")
			}
		}
	}
	if !found {
		t.Fatal("retro row missing")
	}
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.Cars[18].Position = 17
	if !a.needsView() {
		t.Fatal("hidden position prevented risk refresh")
	}
	if tone(rowID(a.frame, c)) != "accent" {
		t.Fatal("safe player did not regain highlight")
	}
	for _, session := range []uint32{7, 12} {
		a.frame.SessionType = session
		if tone(rowID(a.frame, c)) == "bad" {
			t.Fatal("final round highlighted")
		}
	}
}
