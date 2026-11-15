package main

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRaceLightsOutSeparatesFormationAndRetainsGridStart(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		m := newModel(RecordingMeta{Format: format, Session: 15})
		m.ingest(eventAt(format, 0, 0, "LGOT"))
		pair := func(frame uint32, clock, distance float32, timer uint32, speed int) {
			lap := lapAt(format, frame, clock, 1, distance)
			lap[73] = 4
			binary.LittleEndian.PutUint32(lap[33:], timer)
			m.ingest(lap)
			m.ingest(traceAt(format, frame, clock, speed))
		}
		pair(1, 0, -116, 0, 0)
		pair(2, 45, 1800, 0, 100)
		pair(3, 142, 5160, 0, 20)
		lights := eventAt(format, 90, 0, "STLG")
		lights[33] = 1
		m.ingest(lights)
		m.ingest(eventAt(format, 100, 0, "LGOT"))
		pair(100, 0, -116, 0, 0)
		pair(101, 5, 10, 5000, 120)
		pair(102, 45, 2600, 45000, 300)
		pair(103, 90, 5274, 90000, 280)
		samples := m.Drivers[0].trace(1)
		if len(samples) != 4 || samples[0].Distance != -116 || samples[3].Time != 90 {
			t.Fatalf("format %d formation mixed into race: %+v", format, samples)
		}
		if warning := traceCoverage(samples, 90050); warning != "" {
			t.Fatal("grid start incorrectly marked missing", warning)
		}
		// Repeated/restart lights-out events must not erase existing timed racing.
		m.ingest(eventAt(format, 100, 0, "LGOT"))
		if len(m.Drivers[0].trace(1)) != 4 {
			t.Fatal("duplicate lights-out erased race data")
		}
	}
	// Qualifying's own reset/attempt logic is unaffected by race-only handling.
	m := newModel(RecordingMeta{Format: 2026, Session: 8})
	m.Drivers[0].Samples = []TraceSample{{Lap: 1, Time: 20, Status: 1, Distance: 100, LapMS: 0}}
	m.ingest(eventAt(2026, 100, 0, "LGOT"))
	if len(m.Drivers[0].Samples) != 1 {
		t.Fatal("cleared qualifying data")
	}
}
func TestSavedMcLarenGameFormattingAndLiteralNames(t *testing.T) {
	raw := "{o:upper}M{/o}{o:lower}c{/o}{o:upper}L{/o}aren Mastercard Formula 1 Team"
	if got := cleanTeamName(raw); got != "McLaren Mastercard Formula 1 Team" {
		t.Fatal(got)
	}
	if got := cleanTeamName("{o:upper}abc{/o} {o:lower}DEF{/o}"); got != "ABC def" {
		t.Fatal(got)
	}
	m := newModel(RecordingMeta{Format: 2026, Teams: &TeamSnapshot{Teams: []CapturedTeam{{Set: 0, ID: 484, RawName: "lng_team_mcl_title_26", Name: raw}}}})
	if got := m.teamName(RecordedDriver{TeamID: 484, TeamKnown: true}); got != "McLaren Mastercard Formula 1 Team" {
		t.Fatal("old snapshot not repaired at display", got)
	}
	if got := capturedTeamDisplayName(CapturedTeam{Name: "Custom <Racing> {Team}"}); got != "Custom <Racing> {Team}" {
		t.Fatal("stripped literal custom name", got)
	}
}
func TestFinalClassificationClosesUnknownStintEnd(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		m := newModel(RecordingMeta{Format: format, Session: 15, State: "recording"})
		d := &m.Drivers[0]
		open := RecordedStint{EndLap: 255, Compound: 18}
		if m.stintEndLabel(d, open, 10) != "In progress" {
			t.Fatal("live stint closed early")
		}
		final := analysisPacket(format, 8, 100, 1700)
		final[29] = 1
		final[31] = 20
		final[51] = 2
		final[52], final[53] = 17, 18
		final[68], final[69] = 9, 255
		m.ingest(final)
		if len(d.Stints) != 2 || m.stintEndLabel(d, d.Stints[1], 10) != "20" {
			t.Fatal("last stint not closed using actual completed laps", d.Stints, d.Completed)
		}
		history := analysisPacket(format, 11, 101, 1701)
		history[31] = 2
		history[1436], history[1437] = 9, 17
		history[1439], history[1440] = 255, 18
		m.ingest(history)
		if m.stintEndLabel(d, d.Stints[1], 10) != "20" {
			t.Fatal("late history reopened final stint")
		}
		d.Classified = false
		m.Meta.State = "completed"
		if got := m.stintEndLabel(d, open, 10); got == "In progress" || got == "20" {
			t.Fatal("invented final stint endpoint without classification", got)
		}
		m.Meta.State = "interrupted"
		if strings.Contains(m.stintEndLabel(d, open, 10), "In progress") {
			t.Fatal("interrupted capture shown live")
		}
	}
}

// Read-only replay of the user's completed race, enabled explicitly. User data
// stays untouched; optional corrected chart output is written to the workspace.
func TestReplayRaceIssuesFromEnvironment(t *testing.T) {
	path := os.Getenv("UDP_TELEMETRY_RACE_REPLAY")
	if path == "" {
		t.Skip("no local race recording configured")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var meta RecordingMeta
	if e = json.Unmarshal(b, &meta); e != nil {
		t.Fatal(e)
	}
	m, e := loadAnalysis(filepath.Dir(path), meta)
	if e != nil {
		t.Fatal(e)
	}
	if m.raceGate.Trigger != "start_lights" || m.raceGate.Frame != 3030 {
		t.Fatal("wrong race capture boundary", m.raceGate)
	}
	checked := 0
	for _, dr := range m.Drivers {
		if dr.Name == "" {
			continue
		}
		lap := dr.trace(1)
		if len(lap) < 100 || lap[0].Frame < 3030 || lap[0].Frame > 3040 || lap[0].Distance > 100 || lap[len(lap)-1].Distance < 5000 {
			t.Fatalf("incomplete start/finish for %s: %d samples %+v", dr.Name, len(lap), lap)
		}
		for _, s := range lap {
			if s.Frame < 3030 {
				t.Fatal("formation data for", dr.Name, s)
			}
		}
		checked++
		t.Logf("%s lap 1: %d samples, first frame %d, distance %.1f–%.1fm", dr.Name, len(lap), lap[0].Frame, lap[0].Distance, lap[len(lap)-1].Distance)
	}
	if checked != 24 {
		t.Fatal("not all 24 drivers checked", checked)
	}
	index := -1
	for i, d := range m.Drivers {
		if d.Name == "BROWNING" {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("BROWNING not found")
	}
	d := &m.Drivers[index]
	samples := d.trace(1)
	if len(samples) < 1500 || samples[0].Distance >= 0 || samples[len(samples)-1].Distance < 5200 || samples[len(samples)-1].Time > 100 {
		t.Fatalf("lap-one coverage: count %d first %+v last %+v", len(samples), samples[0], samples[len(samples)-1])
	}
	for _, s := range samples {
		if s.Time > 5 && s.LapMS == 0 {
			t.Fatal("formation sample remains", s)
		}
	}
	for _, dr := range m.Drivers {
		if dr.TeamID == 484 && m.teamName(dr) != "McLaren Mastercard Formula 1 Team" {
			t.Fatal("McLaren formatting remains", m.teamName(dr))
		}
	}
	if len(d.Stints) != 2 || m.stintEndLabel(d, d.Stints[1], 10) != "20" {
		t.Fatal("final stint", d.Stints, d.Completed)
	}
	t.Logf("BROWNING lap one: %d samples, %.3f–%.3fs, %.1f–%.1fm; final stint ends lap %s", len(samples), samples[0].Time, samples[len(samples)-1].Time, samples[0].Distance, samples[len(samples)-1].Distance, m.stintEndLabel(d, d.Stints[1], 10))
	if out := os.Getenv("UDP_TELEMETRY_RACE_CHART"); out != "" {
		if e := renderTrace(out, samples, nil, 0); e != nil {
			t.Fatal(e)
		}
	}
	if os.Getenv("UDP_TELEMETRY_RACE_FORWARD_TEST") == "1" {
		r, e := openRecorder(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		defer r.finish("interrupted")
		partial, e := readJournal(filepath.Join(filepath.Dir(path), meta.ID+".udp.gz"), func(b []byte) { r.ingest(b, "race-replay") })
		if e != nil || partial || r.current == nil {
			t.Fatal("forward recording", e, partial, r.err)
		}
		if r.current.RaceCaptureFrame == nil || *r.current.RaceCaptureFrame != 3030 || r.current.RaceCaptureTrigger != "start_lights" {
			t.Fatal("forward recorder started at wrong point")
		}
		if e = r.flush(); e != nil {
			t.Fatal(e)
		}
		_, e = readJournal(filepath.Join(r.dir, r.current.ID+".udp.gz"), func(b []byte) {
			if (b[6] == 2 || b[6] == 6) && u32(b[23:]) < 3030 {
				t.Fatal("forward journal saved formation measurements")
			}
		})
		if e != nil {
			t.Fatal(e)
		}
		forward, e := loadAnalysis(r.dir, *r.current)
		if e != nil {
			t.Fatal(e)
		}
		for i := range m.Drivers {
			a, b := m.Drivers[i].trace(1), forward.Drivers[i].trace(1)
			if len(a) != len(b) || len(b) == 0 || a[0].Frame != b[0].Frame || a[len(a)-1].Frame != b[len(b)-1].Frame {
				t.Fatal("forward capture changed lap one for", m.Drivers[i].Name, len(a), len(b))
			}
		}
		t.Logf("Forward recorder retained all 24 lap-one traces from frame 3030, excluded formation measurements, and saved %d of %d original packets", r.current.Packets, meta.Packets)
	}
}
