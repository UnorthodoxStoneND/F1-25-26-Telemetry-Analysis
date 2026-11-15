package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

type fakeTeamMemory map[uintptr][]byte

func (m fakeTeamMemory) read(addr uintptr, n int) []byte {
	for start, b := range m {
		if addr >= start && addr-start <= uintptr(len(b)) && uintptr(n) <= uintptr(len(b))-(addr-start) {
			return append([]byte(nil), b[addr-start:addr-start+uintptr(n)]...)
		}
	}
	return nil
}
func fakeLoadedTeams() (fakeTeamMemory, uintptr, uintptr) {
	base, system := uintptr(0x140000000), uintptr(0x20000000)
	m := fakeTeamMemory{system: make([]byte, 0x300)}
	for set := 0; set < 2; set++ {
		slots := uintptr(0x21000000 + set*0x1000)
		entries := slots + 0x100
		m[slots] = make([]byte, 0x20)
		m[entries] = make([]byte, 32)
		binary.LittleEndian.PutUint64(m[system][0x2c8+set*16:], uint64(slots))
		binary.LittleEndian.PutUint64(m[slots][8:], uint64(entries))
		binary.LittleEndian.PutUint64(m[slots][16:], uint64(entries+32))
		for id := 0; id < 2; id++ {
			record := uintptr(0x22000000 + set*0x10000 + id*0x1000)
			b := make([]byte, 0x768)
			m[record] = b
			binary.LittleEndian.PutUint64(m[entries][id*16+8:], uint64(record))
			binary.LittleEndian.PutUint32(b, uint32(id<<16|set))
			for offset, text := range map[int]string{0x18: "team_label", 0xb0: []string{"lng_team_original", "Custom Racing Ω"}[set], 0x1e0: "Custom short", 0x148: "CUS"} {
				binary.LittleEndian.PutUint64(b[offset-16:], uint64(base+0x5613670))
				binary.LittleEndian.PutUint64(b[offset-8:], 128)
				binary.LittleEndian.PutUint64(b[offset:], uint64(record+uintptr(offset)+8))
				copy(b[offset+8:], text)
			}
		}
	}
	return m, base, system
}
func TestReadAllLoadedTeamRecordsAndRejectBadLayout(t *testing.T) {
	m, base, system := fakeLoadedTeams()
	teams, e := readLoadedTeamTables(m, base, system)
	if e != nil || len(teams) != 4 {
		t.Fatalf("all tables: %v %v", teams, e)
	}
	if teams[2].RawName != "Custom Racing Ω" || teams[3].Set != 1 {
		t.Fatal("custom names lost", teams)
	}
	binary.LittleEndian.PutUint64(m[0x22010000][0xa0:], 123)
	// An invalid name holder cannot produce an invented string.
	teams, e = readLoadedTeamTables(m, base, system)
	if e != nil || teams[2].RawName != "" {
		t.Fatal("accepted invalid name holder", teams, e)
	}
	binary.LittleEndian.PutUint32(m[0x22010000], 0x70001)
	if _, e = readLoadedTeamTables(m, base, system); e == nil {
		t.Fatal("accepted mismatched team key")
	}
}
func TestReadTeamMemoryReadOnly(t *testing.T) {
	h, e := syscall.GetCurrentProcess()
	if e != nil {
		t.Fatal(e)
	}
	b := []byte("read-only team identity test")
	got := (teamMemory{h}).read(uintptr(unsafe.Pointer(&b[0])), len(b))
	if !bytes.Equal(got, b) {
		t.Fatalf("ReadProcessMemory: %q", got)
	}
	if got := (teamMemory{h}).read(0, 10); got != nil {
		t.Fatal("accepted null memory address")
	}
}
func TestLoadedTeamEntrantsIgnoreUnusedSlots(t *testing.T) {
	base, manager, begin := uintptr(0x140000000), uintptr(0x20000000), uintptr(0x21000000)
	m := fakeTeamMemory{manager: make([]byte, 0x300), begin: make([]byte, 0x88*3)}
	binary.LittleEndian.PutUint64(m[manager][0x2e0:], uint64(begin))
	binary.LittleEndian.PutUint64(m[manager][0x2e8:], uint64(begin+0x88*3))
	for i := 0; i < 3; i++ {
		r := m[begin][i*0x88:]
		binary.LittleEndian.PutUint64(r, uint64(base+0x5612e20))
		binary.LittleEndian.PutUint32(r[0x58:], 1)
		binary.LittleEndian.PutUint16(r[0x52:], 44)
		r[0x83] = 22
		r[0x82] = byte(i)
	}
	m[begin][0x82] = 255
	binary.LittleEndian.PutUint64(m[begin][0x88*2:], 0)
	entries := readLoadedTeamEntrants(m, base, manager)
	if len(entries) != 1 || entries[0].Set != 1 || entries[0].DriverID != 44 || entries[0].Number != 22 {
		t.Fatal(entries)
	}
}
func TestTeamNamesBothFormatsAndAmbiguousRewrites(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		s := &TeamSnapshot{Teams: []CapturedTeam{{Set: 0, ID: 3, Name: "Custom Williams"}, {Set: 1, ID: 0, Name: "Walker Genesis Racing"}}}
		m := newModel(RecordingMeta{Format: format, Teams: s})
		if n := m.teamName(RecordedDriver{TeamID: 3, TeamKnown: true}); n != "Custom Williams" {
			t.Fatal(format, n)
		}
		if n := m.teamName(RecordedDriver{MyTeam: true, TeamKnown: true}); n != "Walker Genesis Racing" {
			t.Fatal(format, n)
		}
		s.Teams = append(s.Teams, CapturedTeam{Set: 0, ID: 44, Name: "Extra Custom Team"})
		s.Rewrites = map[string]int{"44": 3}
		s.ByteRewrites = s.Rewrites
		if n := m.teamName(RecordedDriver{TeamID: 3, TeamKnown: true}); n != "—" {
			t.Fatal("guessed ambiguous ancestor", format, n)
		}
		s.Entrants = []CapturedEntrant{{DriverID: 12, Number: 21, Set: 0, TeamID: 44}}
		if n := m.teamName(RecordedDriver{TeamID: 3, TeamKnown: true, DriverID: 12, RaceNumber: 21}); n != "Extra Custom Team" {
			t.Fatal("entrant match", format, n)
		}
		if n := m.teamName(RecordedDriver{TeamID: 3, TeamKnown: true, DriverID: 13, RaceNumber: 21}); n != "—" {
			t.Fatal("matched another driver", format, n)
		}
	}
}
func teamLngFixture(values map[string]string) []byte {
	keys, text, index := []byte{}, []byte{}, []byte{}
	for k, v := range values {
		var pair [8]byte
		binary.BigEndian.PutUint32(pair[:], uint32(len(keys)))
		binary.BigEndian.PutUint32(pair[4:], uint32(len(text)))
		index = append(index, pair[:]...)
		keys = append(keys, append([]byte(k), 0)...)
		text = append(text, append([]byte(v), 0)...)
	}
	b := []byte("LNGT\x00\x00\x00\x00")
	for _, chunk := range []struct {
		tag string
		b   []byte
	}{{"SIDA", index}, {"SIDB", keys}, {"LNGB", text}} {
		b = append(b, chunk.tag...)
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(chunk.b)))
		b = append(b, size[:]...)
		if chunk.tag == "SIDA" {
			b = append(b, 0, 0, 0, byte(len(values)))
		}
		b = append(b, chunk.b...)
	}
	binary.BigEndian.PutUint32(b[4:], uint32(len(b)-8))
	return b
}
func TestTeamLanguageOverridesAndCorruptInput(t *testing.T) {
	b := teamLngFixture(map[string]string{"lng_title": "<b>Original Racing</b>"})
	texts, e := parseTeamLanguage(b)
	if e != nil || resolveTeamText("lng_title", texts) != "Original Racing" {
		t.Fatal(texts, e)
	}
	if resolveTeamText("@lng_missing", texts) != "" || resolveTeamText("Custom Racing Ω", texts) != "Custom Racing Ω" {
		t.Fatal("literal / missing key")
	}
	applyTeamTextOverrides([]byte(`{"lang.string":{"game/lng_title":{"status":"ok","value":{"text":{"*":"All","eng":"Custom Name"}}},"game/lng_other":{"status":"ok","value":{"text":{"*":"Wrong"},"within":{"mode":"other"}}}}}`), "eng", "my_team_career_26", texts)
	if texts["lng_title"] != "Custom Name" || texts["lng_other"] != "" {
		t.Fatal(texts)
	}
	for _, bad := range [][]byte{b[:12], []byte("bad"), append([]byte(nil), b...)} {
		if len(bad) == len(b) {
			binary.BigEndian.PutUint32(bad[20:], 0xffffffff)
		}
		if _, e := parseTeamLanguage(bad); e == nil {
			t.Fatal("accepted malformed language table")
		}
	}
}
func TestInstalledTeamLanguageFixture(t *testing.T) {
	path := os.Getenv("F1_TELEMETRY_LANGUAGE_FIXTURE")
	if path == "" {
		t.Skip("optional installed-game language fixture")
	}
	texts := map[string]string{}
	if e := mergeTeamLanguage(path, texts); e != nil {
		t.Fatal(e)
	}
	if len(texts) < 1000 || resolveTeamText("lng_team_mer_title", texts) == "" || resolveTeamText("lng_team_audi_title", texts) == "" {
		t.Fatal("installed team names did not resolve")
	}
	t.Logf("Decoded %d installed strings; Mercedes: %s; Audi: %s", len(texts), resolveTeamText("lng_team_mer_title", texts), resolveTeamText("lng_team_audi_title", texts))
}
func TestMountedLanguagePriorityAndStorage(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "game")
	put := func(path string, b []byte) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	put(filepath.Join(root, "baseline", "123-"+verifiedTeamBuild[:12], "lang", "language_eng.lng"), teamLngFixture(map[string]string{"lng_title": "Baseline"}))
	put(filepath.Join(game, "swofty", "assets.json"), []byte(`{"lang":{"*":[{"table":"assets/low.lng","priority":0},{"table":"assets/high.lng","priority":5}],"eng":{"table":"assets/eng.lng","priority":1}}}`))
	for _, item := range []struct{ path, value string }{{"low", "Low"}, {"high", "Modded Team"}, {"eng", "English"}} {
		put(filepath.Join(game, "swofty", "assets", item.path+".lng"), teamLngFixture(map[string]string{"lng_title": item.value}))
	}
	texts, lang, warnings := loadTeamLanguage(root, game, "my_team_career_26", time.Now().Add(-time.Minute))
	if lang != "eng" || texts["lng_title"] != "Modded Team" || len(warnings) > 0 {
		t.Fatal(lang, texts, warnings)
	}
	s := &TeamSnapshot{Captured: time.Now(), Teams: []CapturedTeam{{Set: 1, ID: 0, Name: "Saved Custom Team"}}}
	r, e := openRecorder(filepath.Join(root, "Documents", "F1 Game Telemetry"))
	if e != nil {
		t.Fatal(e)
	}
	if e = r.begin(1, 2026, 0, "career"); e != nil {
		t.Fatal(e)
	}
	app := App{rec: r, profileName: "career"}
	id := r.current.ID
	app.analysis.Model = newModel(*r.current)
	app.analysis.Loading, app.analysis.Generation = id, 1
	app.applyTeamCapture("wrong", "career", s, nil)
	if r.current.Teams != nil {
		t.Fatal("cross-session snapshot")
	}
	app.applyTeamCapture(id, "wrong", s, nil)
	if r.current.Teams != nil {
		t.Fatal("cross-profile snapshot")
	}
	app.applyTeamCapture(id, "career", s, nil)
	if app.analysis.Model.Meta.Teams != s || app.analysis.Loading != "" || app.analysis.Generation != 2 {
		t.Fatal("analysis retained stale team metadata")
	}
	r.finish("completed")
	loaded, e := openRecorder(filepath.Dir(r.dir))
	if e != nil || len(loaded.items) != 1 || loaded.items[0].Teams.Teams[0].Name != "Saved Custom Team" {
		t.Fatal(loaded, e)
	}
	m := newModel(loaded.items[0])
	if m.teamName(RecordedDriver{MyTeam: true, TeamKnown: true}) != "Saved Custom Team" {
		t.Fatal("saved name lost")
	}
	app.applyTeamCapture(id, "career", nil, errors.New("late"))
	if loaded.items[0].TeamCaptureError != "" {
		t.Fatal("modified old session")
	}
	encoded, _ := json.Marshal(loaded.items[0])
	if !strings.Contains(string(encoded), "team_snapshot") {
		t.Fatal("metadata not persisted")
	}
}
