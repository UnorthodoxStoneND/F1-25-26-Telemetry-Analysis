package main

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestAnalysisCaptureRewindAndExport(t *testing.T) {
	a := analysisCapture{root: t.TempDir()}
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	f := Frame{Player: 5, State: active | windowReady, SessionType: 1, SessionKind: 1,
		Analysis: AnalysisSession{Run: 23, Attempt: 1, Sample: 100, Track: "silverstone", Career: "career-17", Season: 0, Round: 2},
		Cars: []Car{
			{Vehicle: 5, Known: knownName | knownLaps | knownTyre, Name: "Browning", Compound: 18, Visual: 17, Analysis: AnalysisCar{Known: analysisLap | analysisSpeed, Speed: 21, Team: "myteam"}},
			{Vehicle: 2, Known: knownName | knownLaps, Name: "Leclerc", Analysis: AnalysisCar{Known: analysisLap, Team: "ferrari"}},
		},
	}
	advance := func() {
		now = now.Add(250 * time.Millisecond)
		f.Analysis.Sample += 200
		a.observe("Test Profile", f, now)
		if a.err != nil {
			t.Fatal(a.err)
		}
	}
	advance()
	dir := a.current.dir
	if !strings.Contains(filepath.ToSlash(dir), "/season-001/round-002-") {
		t.Fatal(dir)
	}
	entries, err := a.entries()
	if err != nil || len(entries) != 0 {
		t.Fatalf("active recording exposed: %v %v", entries, err)
	}
	completeLap := func(car *Car, lap, ms uint32, valid bool) {
		car.Laps, car.Last = lap, ms
		car.Known |= knownLast
		car.Analysis.LapFlags = 0
		if valid {
			car.Best = ms
			car.Known |= knownBest
			car.Analysis.LapFlags = 2
		}
	}
	completeLap(&f.Cars[0], 1, 87001, true)
	completeLap(&f.Cars[1], 1, 86200, true)
	advance()
	completeLap(&f.Cars[0], 2, 85000, true)
	completeLap(&f.Cars[1], 2, 86000, true)
	advance()
	f.State |= paused
	f.Cars[0].Laps = 0
	advance()
	f.State &^= paused
	completeLap(&f.Cars[0], 1, 87001, true)
	completeLap(&f.Cars[1], 1, 86200, true)
	advance()
	completeLap(&f.Cars[0], 2, 89000, false)
	advance()
	a.observe("Test Profile", f, now) // duplicate publication
	f.Analysis.Chequered = true
	advance()
	f.State = sessionEnded
	advance()

	rows, err := readAnalysisCSV(dir, "laps.csv", lapsCSV, 64)
	if err != nil || len(rows) != 3 || rows[2][5] != "89000" || rows[2][6] != "false" {
		t.Fatalf("canonical laps after rewind: %v %v", rows, err)
	}
	table, err := practiceRankings(dir)
	if err != nil {
		t.Fatal(err)
	}
	rank := table["table"].(M)["rows"].([][]string)
	if len(rank) != 2 || rank[0][1] != "Leclerc" || rank[1][2] != "My Team" || rank[1][6] != "+0.801" || rank[1][7] != "1:27.001" {
		t.Fatalf("unexpected rankings: %v", rank)
	}
	entries, err = a.entries()
	if err != nil || len(entries) != 1 || entries[0].state != "finished" {
		t.Fatalf("unexpected session list: %v %v", entries, err)
	}
	app := App{enabled: true, analysisEnabled: true, capture: a}
	view := app.analysisView()
	raw, err := json.Marshal(view)
	if err != nil || !bytes.Contains(raw, []byte("practice-rankings")) || !bytes.Contains(raw, []byte("United Kingdom")) {
		t.Fatalf("invalid practice page: %s %v", raw, err)
	}
	effect, err := app.analysisEvent(json.RawMessage(`{"event":{"click":{"id":"analysis-export"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	export := effect.(M)["open"].(string)
	for _, name := range analysisFiles {
		read := func(folder string) []byte {
			result, err := os.ReadFile(filepath.Join(folder, name))
			if err != nil {
				t.Fatal(err)
			}
			return result
		}
		if !bytes.Equal(read(dir), read(export)) {
			t.Fatalf("export changed %s", name)
		}
	}
	samples, err := os.Open(filepath.Join(export, "samples.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer samples.Close()
	trace, err := csv.NewReader(samples).ReadAll()
	if err != nil || len(trace) != 13 || !slices.Equal(trace[0], sampleCSV) || trace[7][1] != "1" || trace[1][24] != "21" {
		t.Fatalf("unexpected trace: %v %v", trace, err)
	}
}

func TestAnalysisCaptureRestartAndPartialSession(t *testing.T) {
	a := analysisCapture{root: t.TempDir()}
	now := time.Now()
	f := Frame{Player: 0, State: active | windowReady, SessionType: 15, SessionKind: 4,
		Analysis: AnalysisSession{Run: 1, Attempt: 1, Sample: 1, Season: -1, Round: -1},
		Cars:     []Car{{Known: knownLaps | knownLast, Laps: 4, Last: 90000, Analysis: AnalysisCar{Known: analysisLap, LapFlags: 2}}},
	}
	a.observe("profile", f, now)
	if a.current == nil || !a.current.partial {
		t.Fatalf("late-start coverage missing: %v", a.err)
	}
	f.State = sessionEnded
	f.Analysis.EndReason = 2
	a.observe("profile", f, now)
	f.State = active | windowReady
	f.Analysis.Attempt++
	f.Analysis.Sample++
	a.observe("profile", f, now.Add(time.Second))
	a.close("interrupted", "profile switched", now.Add(2*time.Second))
	entries, err := a.entries()
	if err != nil || len(entries) != 2 || entries[0].label != "Sprint" {
		t.Fatalf("restart did not create separate attempts: %v %v", entries, err)
	}
	for _, entry := range entries {
		if entry.state != "interrupted" || entry.practice || entry.coverage != "partial" {
			t.Fatalf("incorrect capture status: %+v", entry)
		}
	}
}

func TestAnalysisWireFollowsVehicleThroughClassification(t *testing.T) {
	b := fixture()
	binary.LittleEndian.PutUint32(b[256:], 5)
	binary.LittleEndian.PutUint64(b[analysisOffset:], 17)
	b[analysisOffset+39] = byte(redFlagSuspended)
	copy(b[analysisOffset+40:], "bahrain")
	at := analysisCarOffset + 5*analysisCarSize
	binary.LittleEndian.PutUint32(b[at:], analysisLap)
	binary.LittleEndian.PutUint32(b[at+4:], 7132)
	copy(b[at+56:], "mclaren")
	f, err := decode(b)
	if err != nil || len(f.Cars) != 1 || f.Analysis.Run != 17 || f.Analysis.Track != "bahrain" || f.Analysis.RedFlag != redFlagSuspended ||
		f.Cars[0].Analysis.Team != "mclaren" || f.Cars[0].Analysis.Current != 7132 {
		t.Fatalf("could not decode analysis data: %+v %v", f, err)
	}
}

func TestAnalysisRewindKeepsCompletedLapAcrossBackwardSamples(t *testing.T) {
	r := analysisRecording{startSample: 200, samples: csv.NewWriter(io.Discard)}
	player := Car{Vehicle: 5, Known: knownLaps | knownTyre, Compound: 18, Visual: 17,
		Analysis: AnalysisCar{Known: analysisProgress | analysisLap}}
	f := Frame{Player: 5, Cars: []Car{player}}
	sample := func(at uint64, clock uint32, distance float32) {
		f.Analysis.Sample = at
		player.Analysis.Current, player.Analysis.LapDistance = clock, distance
		f.Cars[0] = player
		if err := r.record(f); err != nil {
			t.Fatal(err)
		}
	}
	sample(200, 139044, 5504)
	sample(400, 50, 10)
	if r.epoch != 0 {
		t.Fatal("normal out-lap line crossing split the epoch")
	}
	player.Laps = 1
	player.Last = 100020
	player.Known |= knownLast
	player.Analysis.LapFlags = 2
	sample(600, 100, 12)
	d := &r.drivers[5]
	first := d.recorded[1]
	if first.actual != "18" || first.visual != "17" {
		t.Fatalf("tyre not recorded: %+v", first)
	}
	sample(800, 78000, 4000)
	player.Laps = 2
	player.Last = 101000
	sample(1000, 750, 22)
	player.Laps = 1
	player.Last = 100020
	sample(1200, 97000, 4900)
	sample(1400, 93800, 4750)
	sample(1600, 90400, 4510)
	if r.epoch != 1 || len(d.recorded) != 1 || d.recorded[1] != first {
		t.Fatalf("rewind corrupted surviving lap: epoch %d, %+v", r.epoch, d.recorded)
	}
	sample(2000, 91000, 4600)
	sample(2600, 92000, 4650)
	sample(2800, 87000, 4400)
	if r.epoch != 2 || d.recorded[1] != first {
		t.Fatalf("second Flashback was not split separately: epoch %d", r.epoch)
	}
}

func TestAnalysisOverviewNames(t *testing.T) {
	country, circuit := analysisCircuit("korea")
	if country != "South Korea" || circuit != "Korea International Circuit" {
		t.Fatalf("Korea banner: %s, %s", country, circuit)
	}
	for key, want := range map[string]string{
		"aston_martin_26": "Aston Martin", "gm_cadillac_26": "Cadillac", "myteam": "My Team", "mod_team": "mod team",
	} {
		if got := analysisTeam(key); got != want {
			t.Errorf("%s: %s, want %s", key, got, want)
		}
	}
}

func TestAnalysisRecordsRedFlagsWithoutVisibleRows(t *testing.T) {
	a := analysisCapture{root: t.TempDir()}
	now := time.Now()
	f := Frame{
		Tick: 200, State: active | windowReady, SessionKind: 3, SessionType: 15,
		Analysis: AnalysisSession{Run: 11, Attempt: 2, Sample: 200, RedFlag: redFlagNone},
		Cars:     []Car{{Name: "Driver", Known: knownName}},
	}
	a.observe("test", f, now)
	if a.current == nil {
		t.Fatalf("recording did not start: %v", a.err)
	}
	dir := a.current.dir
	observe := func(tick uint64, state uint32, flag redFlagStatus) {
		t.Helper()
		f.Tick, f.State, f.Analysis.RedFlag = tick, state, flag
		if state == active|windowReady {
			f.Analysis.Sample = tick
		} else {
			f.Analysis.Sample = 0
		}
		a.observe("test", f, now.Add(time.Duration(tick)*time.Millisecond))
		if a.err != nil {
			t.Fatal(a.err)
		}
	}
	observe(400, active|windowReady, redFlagDeployed)
	f.Cars = nil
	observe(600, paused|active, redFlagSuspended) // paused with no fresh driver sample
	observe(800, 0, redFlagSuspended)             // flow gate also clears the vehicle table
	observe(1000, 0, redFlagUnavailable)
	observe(1200, 0, redFlagNone) // resume is reported before the window opens again
	observe(7000, 0, redFlagNone) // flags remain readable at the normal checkpoint
	got, err := readAnalysisCSV(dir, flagsFileName, flagsCSV, 16)
	want := [][]string{{"0", "none"}, {"200", "deployed"}, {"400", "suspended"}, {"800", "unavailable"}, {"1000", "none"}}
	if err != nil || !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Fatalf("flag transitions: %v, %v", got, err)
	}
	f.Analysis.Chequered = true
	f.Cars = []Car{{Name: "Driver", Known: knownName}}
	observe(7200, active|windowReady, redFlagNone)
	observe(7400, sessionEnded, redFlagUnavailable)
	if a.current != nil {
		t.Fatal("recording did not close")
	}
	timed, err := readAnalysisCSV(dir, "samples.csv", sampleCSV, 16)
	if err != nil || len(timed) != 3 || timed[1][len(sampleCSV)-1] != "deployed" {
		t.Fatalf("flag column or vehicle sample gating changed: %v, %v", timed, err)
	}
}

func TestAnalysisCanExportLegacyRecordings(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Recordings", "old-session")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	row := make([]string, len(sessionCSV))
	row[0], row[6], row[11] = "1", "Practice 1", "finished"
	if err := writeAnalysisCSV(dir, "session.csv", [][]string{sessionCSV, row}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"drivers.csv", "laps.csv", "samples.csv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("legacy"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a := analysisCapture{root: root}
	entries, err := a.entries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("could not read legacy session: %v, %v", entries, err)
	}
	exported, err := exportAnalysis(root, entries[0])
	if err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(exported)
	if err != nil || len(files) != 4 {
		t.Fatalf("legacy export: %v, %v", files, err)
	}
}
