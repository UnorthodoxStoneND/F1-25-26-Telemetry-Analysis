package main

import (
	"encoding/binary"
	"slices"
	"testing"
)

func TestLiveStateClassificationOrderAndFallback(t *testing.T) {
	f := grid()
	slices.Reverse(f.Cars)
	f.State |= classificationOrder
	full := visible(f, "full")
	if full[0].Vehicle != 23 || full[23].Vehicle != 0 {
		t.Fatal("classification array was re-sorted")
	}
	near := visible(f, "nearest")
	for i, c := range near {
		if c.Vehicle != uint32(14-i) {
			t.Fatal("nearby selection ignored classification", i, c.Vehicle)
		}
	}
	f.State &^= classificationOrder
	if visible(f, "full")[0].Vehicle != 0 || visible(f, "nearest")[0].Vehicle != 10 {
		t.Fatal("position fallback changed")
	}
	for i := range f.Cars {
		f.Cars[i].Known &^= knownPosition
		f.Cars[i].Known |= knownGrid
		f.Cars[i].Grid = int32(24 - f.Cars[i].Vehicle)
	}
	if visible(f, "full")[0].Vehicle != 23 {
		t.Fatal("grid fallback changed")
	}
	for i := range f.Cars {
		f.Cars[i].Known &^= knownGrid
	}
	if visible(f, "full")[0].Vehicle != 0 {
		t.Fatal("vehicle fallback changed")
	}
	b := fixture()
	binary.LittleEndian.PutUint32(b[40:], active|classificationOrder)
	decoded, err := decode(b)
	if err != nil || decoded.State&classificationOrder == 0 {
		t.Fatal("classification flag lost across ABI", err)
	}
}
func TestClassificationOrderSharedViewsAndRefresh(t *testing.T) {
	a := App{frame: grid(), mode: "full"}
	a.frame.State |= classificationOrder
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.Cars[0], a.frame.Cars[1] = a.frame.Cars[1], a.frame.Cars[0]
	if !a.needsView() {
		t.Fatal("order-only change did not refresh")
	}
	m := a.retroModel(true)
	if m.Rows[0].ID != rowID(a.frame, a.frame.Cars[0]) {
		t.Fatal("retro view re-sorted order")
	}
	blocks := a.view()["body"].(M)["blocks"].([]M)
	rows := blocks[len(blocks)-1]["table"].(M)["rows"].([]M)
	if rows[0]["id"] != m.Rows[0].ID {
		t.Fatal("launcher and retro orders differ")
	}
}
