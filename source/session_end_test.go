package main

import "testing"

func TestSessionEndWithFrozenOrResetEventFrames(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		for _, closingFrame := range []uint32{100, 0} {
			for _, closingUID := range []uint64{123, 0} {
				var tel Telemetry
				start := packet(format, 3, 123, 100)
				copy(start[29:], "SSTA")
				tel.ingest(start)
				tel.Count = 1
				tel.Cars[0] = Car{Participant: true, Name: "Old session"}
				button := packet(format, 3, 123, 100)
				copy(button[29:], "BUTN")
				tel.ingest(button)
				end := packet(format, 3, closingUID, closingFrame)
				copy(end[29:], "SEND")
				if !tel.ingest(end) || tel.Active || !tel.Ended || tel.Cars[0].Name != "" {
					t.Fatalf("end lost: format %d frame %d uid %d", format, closingFrame, closingUID)
				}
				if tel.ingest(packet(format, 7, 123, 101)) || tel.Active {
					t.Fatal("late old data revived session")
				}
				if tel.ingest(packet(format, 7, 0, 0)) || tel.Active {
					t.Fatal("late menu data revived session")
				}
				// Same-UID restarts can legitimately begin at the same frozen frame.
				if !tel.ingest(start) || !tel.Active {
					t.Fatal("explicit restart failed")
				}
			}
		}
	}
}

func TestEndFromOldSessionCannotClearNewSession(t *testing.T) {
	var tel Telemetry
	tel.ingest(packet(2026, 7, 1, 100))
	tel.ingest(packet(2026, 7, 2, 200))
	end := packet(2026, 3, 1, 999)
	copy(end[29:], "SEND")
	if tel.ingest(end) || !tel.Active || tel.UID != 2 {
		t.Fatal("old end cleared new session")
	}
}

func TestExplicitMenuMetadataEndsEstablishedSession(t *testing.T) {
	var tel Telemetry
	session := packet(2026, 1, 123, 10)
	session[35], session[36] = 15, 4
	tel.ingest(session)
	tel.Cars[0] = Car{Participant: true, Name: "Old session"}
	menu := packet(2026, 1, 0, 0)
	menu[36] = 255
	if !tel.ingest(menu) || tel.Active || tel.Cars[0].Name != "" {
		t.Fatal("explicit menu state retained old session")
	}
}
