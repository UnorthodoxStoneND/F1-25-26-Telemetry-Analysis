package main

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestSessionLabelsAndPlacement(t *testing.T) {
	want := []string{"Unknown", "Practice 1", "Practice 2", "Practice 3", "Short Practice",
		"Qualifying 1", "Qualifying 2", "Qualifying 3", "Short Qualifying", "One-Shot Qualifying",
		"Sprint Qualifying 1", "Sprint Qualifying 2", "Sprint Qualifying 3", "Short Sprint Qualifying", "One-Shot Sprint Qualifying",
		"Race / Sprint (unconfirmed)", "Race", "Race", "Time Trial"}
	for id, label := range want {
		a := App{frame: grid(), mode: "full"}
		a.frame.SessionType = uint32(id)
		if got := sessionLabel(a.frame); got != label {
			t.Fatalf("type %d: %q, want %q", id, got, label)
		}
		view := a.view()
		blocks := view["body"].(M)["blocks"].([]M)
		if !strings.Contains(view["subtitle"].(string), " · "+label+" · ") || blocks[0]["table"] == nil {
			t.Fatal("session label is not in the existing subtitle above the table")
		}
	}
}

func TestSessionCountersAndRefresh(t *testing.T) {
	b := fixture()
	binary.LittleEndian.PutUint32(b[40:], active|sessionTimer)
	binary.LittleEndian.PutUint32(b[52:], 10)
	binary.LittleEndian.PutUint32(b[60:], 183)
	f, err := decode(b)
	if err != nil || sessionSummary(f) != "Sprint Qualifying 1 · 03:03 remaining" {
		t.Fatalf("timer ABI: %+v %v", f, err)
	}
	a := App{frame: f, mode: "full"}
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.Progress--
	if !a.needsView() || sessionSummary(a.frame) != "Sprint Qualifying 1 · 03:02 remaining" {
		t.Fatal("countdown did not refresh")
	}
	a.frame.State = active | sessionLaps
	a.frame.SessionType = 15
	a.frame.SessionKind = 4
	a.frame.Progress = 18<<16 | 7
	if sessionSummary(a.frame) != "Sprint · Lap 7/18" {
		t.Fatal(sessionSummary(a.frame))
	}
	a.frame.Progress = 18<<16 | 19
	if sessionSummary(a.frame) != "Sprint" {
		t.Fatal("invalid lap count was displayed")
	}
	a.frame.State = active | sessionTimer
	a.frame.SessionType = 5
	a.frame.Progress = 0
	if sessionSummary(a.frame) != "Qualifying 1 · 00:00 remaining" {
		t.Fatal("zero timer confused with unknown")
	}
	a.frame.State = active
	if sessionSummary(a.frame) != "Qualifying 1" {
		t.Fatal("unknown countdown was fabricated")
	}
}

func TestSprintRaceAndLifecycle(t *testing.T) {
	b := fixture()
	binary.LittleEndian.PutUint32(b[52:], 15)
	binary.LittleEndian.PutUint32(b[56:], 4)
	f, err := decode(b)
	if err != nil || sessionLabel(f) != "Sprint" {
		t.Fatalf("Sprint ABI: %+v %v", f, err)
	}
	a := App{frame: f, mode: "full"}
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.SessionKind = 3
	if sessionLabel(a.frame) != "Race" || !a.needsView() {
		t.Fatal("Sprint to Race did not refresh")
	}
	a.lastView = a.viewState()
	a.frame.SessionKind = 0
	if sessionLabel(a.frame) != "Race / Sprint (unconfirmed)" || !a.needsView() {
		t.Fatal("lost context retained Race label")
	}
	a.frame.SessionType = 0
	a.frame.SessionKind = 4
	if sessionLabel(a.frame) != "Sprint" {
		t.Fatal("context-only Sprint unavailable")
	}
	a.frame.SessionType = 16
	if sessionLabel(a.frame) != "Race" {
		t.Fatal("Sprint context leaked into race_2")
	}
	a.frame.State = 0
	if sessionLabel(a.frame) != "" {
		t.Fatal("session label survived session end")
	}
	if _, ok := a.view()["body"].(M)["blocks"].([]M)[0]["text"]; ok {
		t.Fatal("inactive view retained heading")
	}
}
