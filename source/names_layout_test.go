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
