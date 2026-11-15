package main

import (
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
)

// Both 2025 and 2026 retain the weekend schedule at these Session offsets.
func packetWeekendStructure(p []byte) []int {
	if len(p) < 716 || p[703] == 0 || p[703] > 12 {
		return nil
	}
	out := make([]int, int(p[703]))
	for i := range out {
		out[i] = int(p[704+i])
		if out[i] < 1 || out[i] > 18 {
			return nil
		}
	}
	return out
}

// Older metadata lacks the schedule, but the original Session packet retains
// it. Read only a bounded prefix, without replaying the whole race or rewriting
// its journal. Missing schedules retain the original generic session label.
func journalWeekendStructure(path string) []int {
	f, e := os.Open(path)
	if e != nil {
		return nil
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		return nil
	}
	defer z.Close()
	reader := io.LimitReader(z, 16<<20)
	var size [2]byte
	for n := 0; n < 4096; n++ {
		if _, e = io.ReadFull(reader, size[:]); e != nil {
			return nil
		}
		packet := make([]byte, int(binary.LittleEndian.Uint16(size[:])))
		if _, e = io.ReadFull(reader, packet); e != nil {
			return nil
		}
		if len(packet) >= 29 && packet[6] == 1 {
			format := binary.LittleEndian.Uint16(packet[:2])
			if format != 2025 && format != 2026 {
				continue
			}
			if structure := packetWeekendStructure(packet[29:]); len(structure) > 0 {
				return structure
			}
		}
	}
	return nil
}

func recordingSessionLabel(meta RecordingMeta) string {
	// F1 sprint weekends can use Race (15) for the Sprint and Race 2 (16)
	// for the main race. Require the explicit sprint-qualifying schedule;
	// do not infer identity from lap count, duration or real-world calendar.
	sprintQual := false
	sprint, main := 0, 0
	for _, kind := range meta.WeekendStructure {
		if kind >= 10 && kind <= 14 {
			sprintQual = true
		}
		if raceSession(kind) && sprintQual {
			if sprint == 0 {
				sprint = kind
			} else {
				main = kind
			}
		}
	}
	if sprint != 0 && main != 0 && sprint != main {
		if meta.Session == sprint {
			return "Sprint"
		}
		if meta.Session == main {
			return "Race"
		}
	}
	return sessionLabel(meta.Session)
}
