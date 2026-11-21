package main

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestTyreAgeMigratesFormerStintPreferences(t *testing.T) {
	a := App{frame: grid(), mode: "full", data: t.TempDir(), profileName: "main"}
	raw := `{"version":1,"hidden":{"race":["stint","last"]},"widths":{"race":{"stint":91,"driver":125}}}`
	if err := os.WriteFile(a.preferencePath(), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	a.loadColumns()
	if a.fieldVisible(layoutRace, "tyre_age") || a.fieldVisible(layoutRace, "last") || a.widths[layoutRace]["tyre_age"] != 91 || a.widths[layoutRace]["driver"] != 125 {
		t.Fatal("former preferences lost", a.hidden, a.widths)
	}
	if err := a.saveColumns(a.hidden); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(a.preferencePath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), `"stint"`) || !strings.Contains(string(saved), `"tyre_age"`) {
		t.Fatal("old column key persisted", string(saved))
	}
}

func TestTyreAgeRejectsFormerStintPublisher(t *testing.T) {
	b := fixture()
	copy(b, "USTLIVE4")
	binary.LittleEndian.PutUint32(b[8:], 4)
	if _, err := decode(b); err == nil {
		t.Fatal("old stint interpreted as tyre age")
	}
}

func TestTyreAgeColumnAndSavedVisibility(t *testing.T) {
	a := App{frame: grid(), mode: "full", data: t.TempDir(), profileName: "main"}
	a.loadColumns()
	for _, session := range []uint32{15, 16, 17} {
		a.frame.SessionType = session
		c := &a.frame.Cars[0]
		c.Known |= knownTyreAge
		c.TyreAgeLaps = 0
		cells := rowCells(t, &a, 0)
		if got := cells["tyre_age"].(M); got["text"] != "0" || got["sort"] != uint32(0) {
			t.Fatal("zero tyre age is not a numeric-sorted text cell", got)
		}
		cols := columns(layoutRace)
		for i, col := range cols {
			if col["key"] == "tyre" && cols[i+1]["key"] != "tyre_age" {
				t.Fatal("tyre age is not immediately after tyre")
			}
		}
		a.lastView, a.viewSent = a.viewState(), true
		c.TyreAgeLaps = 7
		if !a.needsView() || cellText(t, rowCells(t, &a, 0)["tyre_age"]) != "7" {
			t.Fatal("tyre age progress did not refresh")
		}
		c.Known &^= knownTyreAge
		if cellText(t, rowCells(t, &a, 0)["tyre_age"]) != "—" {
			t.Fatal("unknown tyre age fabricated a count")
		}
	}
	clickColumns(t, &a, "columns")
	if _, err := submitColumns(&a, "race", `{"tyre_age":false,"last":false}`); err != nil {
		t.Fatal(err)
	}
	if _, ok := rowCells(t, &a, 0)["tyre_age"]; ok {
		t.Fatal("hidden tyre age still displayed")
	}
	b := App{frame: a.frame, mode: "full", data: a.data, profileName: "main"}
	b.loadColumns()
	if b.fieldVisible(layoutRace, "tyre_age") || b.fieldVisible(layoutRace, "last") {
		t.Fatal("saved selection not retained")
	}
	b.lastView, b.viewSent = b.viewState(), true
	b.frame.Cars[0].Known |= knownTyreAge
	b.frame.Cars[0].TyreAgeLaps = 20
	if b.needsView() {
		t.Fatal("hidden tyre age invalidated view")
	}
	for _, session := range []uint32{1, 5, 10} {
		b.frame.SessionType = session
		if _, ok := rowCells(t, &b, 0)["tyre_age"]; ok {
			t.Fatal("tyre age leaked into timed view")
		}
	}
}
func TestTyreAgeWireValidation(t *testing.T) {
	b := fixture()
	binary.LittleEndian.PutUint32(b[260:], knownTyreAge)
	binary.LittleEndian.PutUint32(b[556:], 9)
	f, err := decode(b)
	if err != nil || f.Cars[0].TyreAgeLaps != 9 || f.Cars[0].Known&knownTyreAge == 0 {
		t.Fatal("tyre age wire mismatch", err)
	}
	binary.LittleEndian.PutUint32(b[556:], 1000)
	f, err = decode(b)
	if err != nil || f.Cars[0].Known&knownTyreAge != 0 {
		t.Fatal("invalid tyre age accepted", err)
	}
}
func TestOlderColumnPreferencesKeepKeysAfterTyreAgeAdded(t *testing.T) {
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
	if !a.fieldVisible(layoutRace, "tyre_age") || a.fieldVisible(layoutRace, "gap") || a.fieldVisible(layoutRace, "last") || a.fieldVisible(layoutRace, "FL") {
		t.Fatal("new column shifted saved field choices")
	}
	raw, _ := json.Marshal(a.view())
	if strings.Contains(string(raw), `"title":"Gap ahead"`) {
		t.Fatal("old hidden gap reappeared")
	}
}
