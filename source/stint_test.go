package main

import (
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestStintColumnAndSavedVisibility(t *testing.T) {
	a := App{frame: grid(), mode: "full", data: t.TempDir(), profileName: "main"}
	a.loadColumns()
	for _, session := range []uint32{15, 16, 17} {
		a.frame.SessionType = session
		c := &a.frame.Cars[0]
		c.Known |= knownStint
		c.StintLaps = 0
		cells := rowCells(t, &a, 0)
		if got := cells["stint"].(M); got["text"] != "0" || got["sort"] != uint32(0) {
			t.Fatal("zero stint is not a numeric-sorted text cell", got)
		}
		cols := columns(layoutRace)
		for i, col := range cols {
			if col["key"] == "tyre" && cols[i+1]["key"] != "stint" {
				t.Fatal("stint is not immediately after tyre")
			}
		}
		a.lastView, a.viewSent = a.viewState(), true
		c.StintLaps = 7
		if !a.needsView() || cellText(t, rowCells(t, &a, 0)["stint"]) != "7" {
			t.Fatal("stint progress did not refresh")
		}
		c.Known &^= knownStint
		if cellText(t, rowCells(t, &a, 0)["stint"]) != "—" {
			t.Fatal("unknown stint fabricated a count")
		}
	}
	clickColumns(t, &a, "columns")
	if _, err := submitColumns(&a, "race", `{"stint":false,"last":false}`); err != nil {
		t.Fatal(err)
	}
	if _, ok := rowCells(t, &a, 0)["stint"]; ok {
		t.Fatal("hidden stint still displayed")
	}
	b := App{frame: a.frame, mode: "full", data: a.data, profileName: "main"}
	b.loadColumns()
	if b.fieldVisible(layoutRace, "stint") || b.fieldVisible(layoutRace, "last") {
		t.Fatal("saved selection not retained")
	}
	b.lastView, b.viewSent = b.viewState(), true
	b.frame.Cars[0].Known |= knownStint
	b.frame.Cars[0].StintLaps = 20
	if b.needsView() {
		t.Fatal("hidden stint invalidated view")
	}
	for _, session := range []uint32{1, 5, 10} {
		b.frame.SessionType = session
		if _, ok := rowCells(t, &b, 0)["stint"]; ok {
			t.Fatal("stint leaked into timed view")
		}
	}
}
func TestStintWireValidation(t *testing.T) {
	b := fixture()
	binary.LittleEndian.PutUint32(b[260:], knownStint)
	binary.LittleEndian.PutUint32(b[556:], 9)
	f, err := decode(b)
	if err != nil || f.Cars[0].StintLaps != 9 || f.Cars[0].Known&knownStint == 0 {
		t.Fatal("stint wire mismatch", err)
	}
	binary.LittleEndian.PutUint32(b[556:], 1000)
	f, err = decode(b)
	if err != nil || f.Cars[0].Known&knownStint != 0 {
		t.Fatal("invalid stint accepted", err)
	}
}
func TestOlderColumnPreferencesKeepKeysAfterStintAdded(t *testing.T) {
	a := App{frame: grid(), mode: "full", data: t.TempDir(), profileName: "main"}
	a.loadColumns()
	// Existing preferences save stable field keys, not the old numeric positions.
	var old columnPreferences
	if err := json.Unmarshal([]byte(`{"version":1,"hidden":{"race":["gap","last","FL"]}}`), &old); err != nil {
		t.Fatal(err)
	}
	hidden := [3]uint32{}
	for _, key := range old.Hidden["race"] {
		hidden[2] |= fieldBit(layoutRace, key)
	}
	if err := a.saveColumns(hidden); err != nil {
		t.Fatal(err)
	}
	a.columnPath = ""
	a.loadColumns()
	if !a.fieldVisible(layoutRace, "stint") || a.fieldVisible(layoutRace, "gap") || a.fieldVisible(layoutRace, "last") || a.fieldVisible(layoutRace, "FL") {
		t.Fatal("new column shifted saved field choices")
	}
	raw, _ := json.Marshal(a.view())
	if strings.Contains(string(raw), `"title":"Gap ahead"`) {
		t.Fatal("old hidden gap reappeared")
	}
}
