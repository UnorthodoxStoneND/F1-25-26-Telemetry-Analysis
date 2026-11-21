package main

import (
	"encoding/binary"
	"path/filepath"
	"testing"
)

func TestPendingPenaltyCells(t *testing.T) {
	for _, tt := range []struct {
		dt, sg uint8
		known  bool
		want   string
	}{{0, 0, true, "—"}, {1, 0, true, "DT"}, {0, 1, true, "SG"}, {2, 3, true, "2DT+3SG"}, {2, 1, false, "—"}} {
		c := Car{DriveThrough: tt.dt, StopGo: tt.sg}
		if tt.known {
			c.Known = knownPenalties
		}
		if got := cellText(t, penaltyCell(c)); got != tt.want {
			t.Fatalf("%+v: %s", tt, got)
		}
	}
	b := fixture()
	binary.LittleEndian.PutUint32(b[260:], knownPenalties)
	b[256+316], b[256+317] = 2, 3
	f, err := decode(b)
	if err != nil || f.Cars[0].DriveThrough != 2 || f.Cars[0].StopGo != 3 || f.Cars[0].Known&knownPenalties == 0 {
		t.Fatal("penalty ABI", err)
	}
}
func TestPenaltyViewsRefreshAndPreferences(t *testing.T) {
	for _, session := range []uint32{5, 6, 7, 10, 11, 12, 15, 16, 17} {
		a := App{frame: grid(), mode: "full", data: t.TempDir(), profileName: "test"}
		a.frame.SessionType = session
		layout := layoutFor(a.frame)
		c := &a.frame.Cars[0]
		c.Known |= knownPenalties
		c.DriveThrough = 1
		if cellText(t, rowCells(t, &a, 0)["pen"]) != "DT" {
			t.Fatal("PEN missing", session)
		}
		a.lastView, a.viewSent = a.viewState(), true
		c.DriveThrough = 0
		if !a.needsView() || cellText(t, rowCells(t, &a, 0)["pen"]) != "—" {
			t.Fatal("served penalty did not clear", session)
		}
		hidden := a.hidden
		hidden[layout] |= fieldBit(layout, "pen")
		if err := a.saveColumns(hidden); err != nil {
			t.Fatal(err)
		}
		restored := App{data: a.data, profileName: a.profileName}
		restored.loadColumns()
		if restored.fieldVisible(layout, "pen") {
			t.Fatal("hidden PEN not restored", filepath.Base(restored.preferencePath()))
		}
		a.lastView = a.viewState()
		c.DriveThrough = 3
		if a.needsView() {
			t.Fatal("hidden PEN caused refresh", session)
		}
	}
	a := App{frame: grid(), mode: "full"}
	a.frame.SessionType = 1
	if _, ok := rowCells(t, &a, 0)["pen"]; ok {
		t.Fatal("practice gained PEN")
	}
}

func TestTimePenaltyTotalAndCombinedCells(t *testing.T) {
	for _, tt := range []struct {
		ms   uint32
		want string
	}{{0, "DT"}, {5000, "5s+DT"}, {5250, "5.25s+DT"}, {30000, "30s+DT"}} {
		c := Car{Known: knownTimePenalty | knownPenalties, TimePenaltyMS: tt.ms, DriveThrough: 1}
		if cellText(t, penaltyCell(c)) != tt.want {
			t.Fatal("time total format", tt)
		}
	}
	b := fixture()
	binary.LittleEndian.PutUint32(b[260:], knownTimePenalty)
	binary.LittleEndian.PutUint32(b[256+320:], 5250)
	f, err := decode(b)
	if err != nil || f.Cars[0].TimePenaltyMS != 5250 || f.Cars[0].Known&knownTimePenalty == 0 {
		t.Fatal("time ABI", err)
	}
	binary.LittleEndian.PutUint32(b[256+320:], 3600001)
	f, err = decode(b)
	if err != nil || f.Cars[0].Known&knownTimePenalty != 0 {
		t.Fatal("invalid time total accepted", err)
	}
	a := App{frame: grid(), mode: "full"}
	a.frame.SessionType = 5
	c := &a.frame.Cars[0]
	c.Known |= knownTimePenalty
	c.TimePenaltyMS = 5000
	a.lastView, a.viewSent = a.viewState(), true
	c.TimePenaltyMS = 10000
	if !a.needsView() || cellText(t, rowCells(t, &a, 0)["pen"]) != "10s" {
		t.Fatal("time penalty did not refresh")
	}
	a.hidden[layoutQualifying] |= fieldBit(layoutQualifying, "pen")
	a.lastView = a.viewState()
	c.TimePenaltyMS = 15000
	if a.needsView() {
		t.Fatal("hidden penalty caused refresh")
	}
}
