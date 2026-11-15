package main

import (
	"encoding/binary"
	"testing"
)

func TestStartingGridBeforeStartEvent(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		var tel Telemetry
		initial := packet(format, 1, 500, 1)
		initial[36] = 255
		initial[27] = 255
		tel.ingest(initial)
		if tel.Ended {
			t.Fatal("loading metadata latched session end")
		}
		entrants := packet(format, 4, 500, 2)
		entrants[29] = 6
		entrants[27] = 255
		tel.ingest(entrants)
		if len(tel.visible("nearest")) != 6 {
			t.Fatal("unknown player hid pre-grid participants")
		}
		grid := packet(format, 2, 500, 3)
		grid[27] = 2
		for i := 0; i < 6; i++ {
			grid[29+i*57+43] = byte(i + 1)
		}
		tel.ingest(grid)
		ids := tel.visible("nearest")
		if len(ids) != 5 || position(tel.Cars[2]) != 3 {
			t.Fatal("grid fallback", ids)
		}
		tel.Cars[2].Name = "Grid driver"
		start := packet(format, 3, 500, 4)
		copy(start[29:], "SSTA")
		start[27] = 255
		tel.ingest(start)
		if tel.Count != 6 || tel.Player != 2 || tel.Cars[2].Name != "Grid driver" {
			t.Fatal("first SSTA discarded grid data")
		}
		binary.LittleEndian.PutUint32(start[23:], 5)
		tel.ingest(start)
		if tel.Count != 0 {
			t.Fatal("subsequent SSTA did not reset restart")
		}
	}
}

func TestInlineSetupAndOptionalHighlight(t *testing.T) {
	a := App{enabled: true, prefs: Preferences{Mode: "full", Port: 20777}}
	a.tel.Active = true
	a.tel.Player = 1
	for i := 0; i < 3; i++ {
		a.tel.Cars[i] = Car{Participant: true, Name: "Driver", Position: i + 1, Setup: make([]byte, 50)}
	}
	a.expandedSet = true
	a.expanded = 1
	blocks := a.telemetryView()["body"].(M)["blocks"].([]M)
	for k, block := range blocks {
		if _, ok := block["card"]; ok {
			above := blocks[k-1]["table"].(M)["rows"].([]M)
			below := blocks[k+1]["table"].(M)["rows"].([]M)
			if above[len(above)-1]["id"] != "car-1" || below[0]["id"] != "car-2" {
				t.Fatal("setup is not below its driver")
			}
			for _, row := range above {
				if _, ok := row["detail"]; ok {
					t.Fatal("native side panel reintroduced")
				}
				if _, ok := row["tone"]; ok {
					t.Fatal("default highlight")
				}
			}
			a.prefs.Highlight = true
			newBlocks := a.telemetryView()["body"].(M)["blocks"].([]M)
			rows := newBlocks[k-1]["table"].(M)["rows"].([]M)
			if rows[1]["tone"] != "accent" {
				t.Fatal("highlight toggle ignored")
			}
			return
		}
	}
	t.Fatal("no inline setup")
}
