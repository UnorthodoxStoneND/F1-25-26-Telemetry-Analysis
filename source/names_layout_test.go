package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"unicode/utf8"
)

func TestPitLaneMarkerAndRefresh(t *testing.T) {
	a := App{frame: grid(), mode: "full", status: "Live"}
	c := &a.frame.Cars[0]
	c.Name = "Norris Lando"
	c.Flags = 6 << 16
	c.Known |= knownPit
	a.lastView, a.viewSent = a.viewState(), true
	c.Flags |= inPitLane
	if !a.needsView() {
		t.Fatal("pit entry did not request a page update")
	}
	data, err := json.Marshal(driverCell(*c))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("NORRIS Lando · PIT")) || !bytes.Contains(data, []byte(`"tone":"warn"`)) {
		t.Fatalf("missing highlighted pit marker: %s", data)
	}
	a.lastView = a.viewState()
	c.Flags &^= inPitLane
	if !a.needsView() {
		t.Fatal("pit exit did not request a page update")
	}
	if driverCell(*c) != "NORRIS Lando" {
		t.Fatal("marker remained after pit exit")
	}
	c.Flags |= inPitLane
	c.Known &^= knownPit
	if driverCell(*c) != "NORRIS Lando" {
		t.Fatal("unknown pit state fabricated a pit marker")
	}
	if driverName(*c) != "NORRIS Lando" {
		t.Fatal("pit state changed the underlying driver name")
	}
}

func TestRetirementOverridesPitAndMutesRow(t *testing.T) {
	for _, tt := range []struct {
		status uint32
		label  string
	}{{4, "DNF"}, {7, "DNF"}, {5, "DSQ"}} {
		a := App{frame: grid(), mode: "full", highlight: true}
		c := &a.frame.Cars[0]
		a.frame.Player = int32(c.Vehicle)
		c.Name = "Norris Lando"
		c.Known |= knownPit | knownName | knownResult
		c.Flags = 6<<16 | inPitLane | 2<<8
		a.lastView, a.viewSent = a.viewState(), true
		c.Flags = (c.Flags &^ resultMask) | tt.status<<8
		if !a.needsView() {
			t.Fatal("retirement did not refresh the table")
		}
		data, err := json.Marshal(driverCell(*c))
		if err != nil || !bytes.Contains(data, []byte("NORRIS Lando · "+tt.label)) || bytes.Contains(data, []byte("PIT")) {
			t.Fatalf("wrong retirement label: %s, %v", data, err)
		}
		blocks := a.view()["body"].(M)["blocks"].([]M)
		row := blocks[0]["table"].(M)["rows"].([]M)[0]
		if row["tone"] != "muted" {
			t.Fatal("retired player row was not muted")
		}
		for _, cell := range row["cells"].([]any) {
			if wear, ok := cell.(wearValue); ok && wear.Tone != "muted" {
				t.Fatal("wear stayed bright")
			}
		}
		c.Known &^= knownResult
		if retirementLabel(*c) != "" {
			t.Fatal("unknown result fabricated retirement")
		}
		c.Known |= knownResult
		c.Flags = (c.Flags &^ resultMask) | 2<<8
		data, _ = json.Marshal(driverCell(*c))
		if !bytes.Contains(data, []byte("PIT")) {
			t.Fatal("active driver lost PIT marker")
		}
	}
}

func TestSurnameThenGivenName(t *testing.T) {
	for _, tt := range []struct{ last, first, want string }{
		{"Norris", "Lando", "NORRIS Lando"},
		{"Button", "Jenson", "BUTTON Jenson"},
		{"Pérez", "Sergio", "PÉREZ Sergio"},
		{"de Vries", "Nyck", "DE VRIES Nyck"},
		{"Custom Surname", "Custom Given", "CUSTOM SURNAME Custom Given"},
	} {
		c := Car{Known: knownName, Name: tt.last + " " + tt.first, Flags: uint32(len(tt.last)) << 16}
		if got := driverName(c); got != tt.want || !utf8.ValidString(got) {
			t.Fatalf("name %q: got %q", c.Name, got)
		}
	}
	for _, name := range []string{"Custom Player", "NORRIS"} {
		if got := driverName(Car{Known: knownName, Name: name}); got != name {
			t.Fatal("fallback name changed", got)
		}
	}
}

func TestColumnsAdaptToAvailableWidth(t *testing.T) {
	flex := map[string]bool{"driver": false, "FL": false, "FR": false, "RL": false, "RR": false, "Avg": false, "last": false, "best": false}
	for _, column := range columns() {
		key := column["key"].(string)
		if _, ok := flex[key]; ok {
			if _, fixed := column["width"]; fixed {
				t.Fatalf("%s cannot expand when the launcher is maximised", key)
			}
			flex[key] = true
		}
	}
	for key, found := range flex {
		if !found {
			t.Fatal("missing responsive column", key)
		}
	}
}
