package main

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
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
	copy(b[analysisOffset+40:], "bahrain")
	at := analysisCarOffset + 5*analysisCarSize
	binary.LittleEndian.PutUint32(b[at:], analysisLap)
	binary.LittleEndian.PutUint32(b[at+4:], 7132)
	copy(b[at+56:], "mclaren")
	f, err := decode(b)
	if err != nil || len(f.Cars) != 1 || f.Analysis.Run != 17 || f.Analysis.Track != "bahrain" ||
		f.Cars[0].Analysis.Team != "mclaren" || f.Cars[0].Analysis.Current != 7132 {
		t.Fatalf("could not decode analysis data: %+v %v", f, err)
	}
}
