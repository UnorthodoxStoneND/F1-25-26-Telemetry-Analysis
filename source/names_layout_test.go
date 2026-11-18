package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPitLaneMarkerAndRefresh(t *testing.T) {
	a := App{frame: grid(), mode: "full", status: "Live"}
	c := &a.frame.Cars[0]
	c.Name = "Norris Lando"
	c.Known |= knownPit
	a.lastView, a.viewSent = a.viewState(), true
	c.Flags |= inPitLane
	if !a.needsView() {
		t.Fatal("pit entry did not request a page update")
	}
	data, err := json.Marshal(statusCell(*c))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("Pit")) || !bytes.Contains(data, []byte(`"tone":"warn"`)) {
		t.Fatalf("missing highlighted pit marker: %s", data)
	}
	a.lastView = a.viewState()
	c.Flags &^= inPitLane
	if !a.needsView() {
		t.Fatal("pit exit did not request a page update")
	}
	if statusCell(*c) != "—" {
		t.Fatal("marker remained after pit exit")
	}
	c.Flags |= inPitLane
	c.Known &^= knownPit
	if statusCell(*c) != "—" {
		t.Fatal("unknown pit state fabricated a pit marker")
	}
	if driverName(*c) != "Norris Lando" {
		t.Fatal("pit state changed the underlying driver name")
	}
}

func TestRetirementOverridesPitAndMutesRow(t *testing.T) {
	for _, tt := range []struct {
		status uint32
		label  string
	}{{4, "DNF"}, {7, "DNF"}, {5, "DSQ"}} {
		a := App{frame: grid(), mode: "full", highlight: true}
		a.frame.SessionType = 15
		c := &a.frame.Cars[0]
		a.frame.Player = int32(c.Vehicle)
		c.Name = "Norris Lando"
		c.Known |= knownPit | knownName | knownResult
		c.WearKnown = 15
		c.Flags = inPitLane | 2<<8
		a.lastView, a.viewSent = a.viewState(), true
		c.Flags = (c.Flags &^ resultMask) | tt.status<<8
		if !a.needsView() {
			t.Fatal("retirement did not refresh the table")
		}
		data, err := json.Marshal(statusCell(*c))
		if err != nil || !bytes.Contains(data, []byte(tt.label)) || bytes.Contains(data, []byte("Pit")) {
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
		data, _ = json.Marshal(statusCell(*c))
		if !bytes.Contains(data, []byte("Pit")) {
			t.Fatal("active driver lost PIT marker")
		}
	}
}

func TestLauncherDisplayName(t *testing.T) {
	for _, name := range []string{"Norris", "Pérez", "de Vries", "Custom Player", "Lando Norris"} {
		if got := driverName(Car{Known: knownName, Name: name}); got != name {
			t.Fatalf("launcher name changed: %q -> %q", name, got)
		}
	}
	if got := driverName(Car{Vehicle: 2}); got != "Driver 3" {
		t.Fatal("missing name did not use the neutral fallback", got)
	}
}

func TestColumnsAdaptToAvailableWidth(t *testing.T) {
	flex := map[string]bool{"driver": false, "FL": false, "FR": false, "RL": false, "RR": false, "Avg": false, "last": false, "best": false}
	for _, column := range columns(layoutRace) {
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
