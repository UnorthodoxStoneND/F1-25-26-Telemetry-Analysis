package main

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"testing"
)

func packet(format int, id byte, uid uint64, frame uint32) []byte {
	n := 22
	if format == 2026 {
		n = 24
	}
	size := map[byte]int{1: 753, 2: 29 + n*57 + 2, 3: 45, 4: 30 + n*57, 5: 33 + n*50, 7: 29 + n*55, 10: 29 + n*46, 11: 1460}[id]
	if format == 2026 {
		switch id {
		case 1:
			size = 926
		case 4:
			size = 30 + n*60
		case 7:
			size = 29 + n*59
		}
	}
	b := make([]byte, size)
	binary.LittleEndian.PutUint16(b, uint16(format))
	b[5] = 1
	b[6] = id
	binary.LittleEndian.PutUint64(b[7:], uid)
	binary.LittleEndian.PutUint32(b[23:], frame)
	b[27] = 0
	return b
}
func putFloat(b []byte, v float32) { binary.LittleEndian.PutUint32(b, math.Float32bits(v)) }

func TestBothFormatsAndMyTeam(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		t.Run(strconv.Itoa(format), func(t *testing.T) {
			var tel Telemetry
			n := 22
			if format == 2026 {
				n = 24
			}
			p := packet(format, 4, 123, 1)
			p[29] = byte(n)
			stride, name, my := 57, 7, 4
			if format == 2026 {
				stride, name, my = 60, 10, 7
			}
			for i := 0; i < n; i++ {
				r := p[30+i*stride:]
				r[0] = 1
				r[my] = 1
				copy(r[name:], "My Team Driver")
			}
			if !tel.ingest(p) || tel.Count != n || !tel.Cars[n-1].MyTeam || tel.Cars[n-1].Name != "My Team Driver" {
				t.Fatal("participants / last slot")
			}
			lap := packet(format, 2, 123, 2)
			for i := 0; i < n; i++ {
				r := lap[29+i*57:]
				binary.LittleEndian.PutUint32(r, 91234)
				r[32] = byte(i + 1)
				r[33] = 3
				r[45] = 2
			}
			tel.ingest(lap)
			status := packet(format, 7, 123, 3)
			ss := 55
			if format == 2026 {
				ss = 59
			}
			for i := 0; i < n; i++ {
				r := status[29+i*ss:]
				r[25], r[26], r[27] = 18, 16, 2
			}
			tel.ingest(status)
			dmg := packet(format, 10, 123, 4)
			for i := 0; i < n; i++ {
				for w := 0; w < 4; w++ {
					putFloat(dmg[29+i*46+w*4:], float32(10+w*10))
				}
			}
			tel.ingest(dmg)
			setup := packet(format, 5, 123, 5)
			for i := 0; i < n; i++ {
				r := setup[29+i*50:]
				r[0], r[1], r[28] = 20, 25, 30
				putFloat(r[37:], 24.5)
				putFloat(r[46:], 55)
			}
			tel.ingest(setup)
			c := tel.Cars[n-1]
			if c.Position != n || c.Last != 91234 || c.Compound != 18 || c.Wear[2] != 30 || len(c.Setup) != 50 {
				t.Fatalf("bad last-slot decode: %+v", c)
			}
			if len(tel.visible("full")) != n || len(tel.visible("none")) != 0 {
				t.Fatal("display modes")
			}
			// First, middle, penultimate and last positions all retain five rows.
			for _, player := range []int{0, 1, 10, n - 2, n - 1} {
				tel.Player = player
				ids := tel.visible("nearest")
				if len(ids) != 5 {
					t.Fatal(ids)
				}
				found := false
				for _, i := range ids {
					found = found || i == player
				}
				if !found {
					t.Fatal("player missing")
				}
			}
			tel.Player = 0
			if !reflect.DeepEqual(tel.visible("nearest"), []int{0, 1, 2, 3, 4}) {
				t.Fatal("leader window")
			}
			tel.Player = n - 1
			if !reflect.DeepEqual(tel.visible("nearest"), []int{n - 5, n - 4, n - 3, n - 2, n - 1}) {
				t.Fatal("last window")
			}
		})
	}
}
func TestHistoryValidity(t *testing.T) {
	var tel Telemetry
	b := packet(2026, 11, 1, 1)
	b[29] = 23
	b[30] = 3
	for i, ms := range []uint32{85000, 80000, 90000} {
		r := b[36+14*i:]
		binary.LittleEndian.PutUint32(r, ms)
		r[13] = 1
	}
	b[36+14+13] = 0 // fastest lap invalid
	tel.ingest(b)
	if tel.Cars[23].Best != 85000 {
		t.Fatal(tel.Cars[23].Best)
	}
	// Another car's earlier frame must not block this rotating history packet.
	b[29] = 0
	binary.LittleEndian.PutUint32(b[23:], 0)
	tel.ingest(b)
	if tel.Cars[0].Best != 85000 {
		t.Fatal("history rotation")
	}
}
func TestPauseEndRestartAndOldDatagrams(t *testing.T) {
	var tel Telemetry
	b := packet(2025, 1, 1, 1)
	b[35] = 15
	b[36] = 4
	b[43] = 1
	tel.ingest(b)
	if !tel.Paused || !tel.Active {
		t.Fatal("paused session")
	}
	tel.Cars[0].Best = 90000
	end := packet(2025, 3, 1, 2)
	copy(end[29:], "SEND")
	tel.ingest(end)
	if tel.Active || tel.Cars[0].Best != 0 {
		t.Fatal("end did not clear")
	}
	if tel.ingest(b) || tel.Active {
		t.Fatal("late packet revived session")
	}
	start := packet(2025, 3, 1, 3)
	copy(start[29:], "SSTA")
	tel.ingest(start)
	if !tel.Active || tel.Ended {
		t.Fatal("same UID restart")
	}
	newSession := packet(2026, 7, 2, 1)
	tel.ingest(newSession)
	if tel.ingest(start) || tel.UID != 2 {
		t.Fatal("old UID restored")
	}
	menu := packet(2026, 1, 2, 4)
	tel.ingest(menu)
	if !tel.Active {
		t.Fatal("unknown startup metadata ended the session")
	}
}
func TestPacketValidationAndReordering(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		for _, id := range []byte{2, 4, 5, 7, 10, 11} {
			b := packet(format, id, 1, 5)
			for size := 0; size < len(b); size++ {
				var tel Telemetry
				if tel.ingest(b[:size]) {
					t.Fatalf("truncated %d %d %d accepted", format, id, size)
				}
			}
			var tel Telemetry
			tel.ingest(b)
			old := packet(format, id, 1, 4)
			if tel.ingest(old) {
				t.Fatal("old frame accepted")
			}
		}
	}
	var tel Telemetry
	b := packet(2026, 10, 1, 1)
	putFloat(b[29:], float32(math.NaN()))
	tel.ingest(b)
	if tel.Cars[0].HasWear {
		t.Fatal("NaN wear")
	}
}
func TestViewAveragePrivacyAndSetups(t *testing.T) {
	a := App{enabled: true, prefs: Preferences{Mode: "full", Port: 20777}, assets: t.TempDir()}
	a.tel.Active = true
	a.tel.Player = 0
	a.tel.Cars[0] = Car{Name: "Player", Participant: true, Position: 1, HasWear: true, Wear: [4]float64{10, 20, 30, 40}, HasStatus: true, Compound: 18, Visual: 16, Setup: make([]byte, 50)}
	a.tel.Cars[1] = Car{Name: "Private human", Participant: true, Position: 2, HasWear: true, Wear: [4]float64{0, 0, 0, 0}}
	b, err := json.Marshal(a.telemetryView())
	if err != nil {
		t.Fatal(err)
	}
	var view M
	json.Unmarshal(b, &view)
	blocks := view["body"].(map[string]any)["blocks"].([]any)
	table := blocks[len(blocks)-1].(map[string]any)["table"].(map[string]any)
	rows := table["rows"].([]any)
	cells := rows[0].(map[string]any)["cells"].([]any)
	if cells[3].(map[string]any)["text"] != "30.0%" || cells[7].(map[string]any)["text"] != "25.0%" || cells[7].(map[string]any)["bar"] != 0.25 {
		t.Fatal(cells)
	}
	private := rows[1].(map[string]any)["cells"].([]any)
	if private[7].(map[string]any)["text"] != "—" {
		t.Fatal("restricted wear shown")
	}
	if len(setupDetail(a.tel.Cars[0])[0]["table"].(M)["rows"].([][]string)) != 11 {
		t.Fatal("setup missing")
	}
	if err := makePills(a.assets); err != nil {
		t.Fatal(err)
	}
}
func TestFlashbackClearsBest(t *testing.T) {
	var tel Telemetry
	b := packet(2025, 2, 1, 1)
	putFloat(b[15:], 100)
	tel.ingest(b)
	tel.Cars[0].Best = 90000
	binary.LittleEndian.PutUint32(b[23:], 2)
	putFloat(b[15:], 90)
	tel.ingest(b)
	if tel.Cars[0].Best != 0 {
		t.Fatal("future best persisted after flashback")
	}
}

func FuzzIngest(f *testing.F) {
	for _, format := range []int{2025, 2026} {
		for _, id := range []byte{1, 2, 3, 4, 5, 7, 10, 11} {
			f.Add(packet(format, id, 1, 1))
		}
	}
	f.Fuzz(func(t *testing.T, b []byte) { var tel Telemetry; tel.ingest(b) })
}
