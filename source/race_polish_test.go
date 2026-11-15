package main

import (
	"reflect"
	"testing"
)

func TestCleanFieldPaceAndTrend(t *testing.T) {
	m := newModel(RecordingMeta{})
	for i, ms := range []uint32{80000, 90000, 100000} {
		m.Drivers[i].Laps[1] = &RecordedLap{Time: ms, Valid: true}
		m.Drivers[i].Laps[2] = &RecordedLap{Time: ms, Valid: i != 2}
	}
	m.Drivers[0].raceLastSet = true
	m.Drivers[0].raceLast = RaceObservation{Lap: 2, Flags: flagPitIn}
	field := cleanField(m)
	if field[1] != (fieldLap{90, 85, 95}) || field[2].Median != 90 {
		t.Fatal("whole-field quantiles or clean-lap filtering", field)
	}
	for _, flag := range []uint8{flagPitIn, flagPitOut, flagSC, flagVSC, flagInvalid} {
		if cleanPaceLap(RaceLapMetric{Time: 90000, Flags: flag}) {
			t.Fatal("non-racing lap entered pace", flag)
		}
	}
	trend := paceTrend([]racePoint{{X: 1, Y: 1}, {X: 2, Y: 2}, {X: 3, Y: 3}, {X: 6, Y: 10}})
	if trend[2].Y != 2 || trend[3].Y != 10 {
		t.Fatal("trend crosses excluded/missing laps", trend)
	}
}
func TestRaceResultsColumnsGapsAndNames(t *testing.T) {
	if clockDuration(3661.2) != "01:01:01" || clockDuration(0) != "—" {
		t.Fatal("race duration formatting")
	}
	m := newModel(RecordingMeta{})
	m.Drivers[0] = RecordedDriver{Classified: true, Position: 1, Completed: 20, RaceSeconds: 1000, ResultStatus: 3}
	m.Drivers[1] = RecordedDriver{Classified: true, Position: 2, Completed: 20, RaceSeconds: 1002, PenaltySeconds: 5, ResultStatus: 3}
	m.Drivers[2] = RecordedDriver{Classified: true, Position: 3, Completed: 19, RaceSeconds: 1001, ResultStatus: 3}
	if m.resultGap(0) != "Winner" || m.resultGap(1) != "+7.000 s" || m.resultGap(2) != "+1 lap" {
		t.Fatal("classified result gaps")
	}
	m.Drivers[2].ResultStatus = 7
	if m.resultGap(2) != "DNF" {
		t.Fatal("retired finisher gap")
	}
	if resultDriverName(RecordedDriver{Name: "NORRIS", RaceNumber: 4}, 0) != "Lando Norris" || resultDriverName(RecordedDriver{Name: "My Driver", RaceNumber: 4}, 0) != "My Driver" || resultDriverName(RecordedDriver{Name: "NORRIS", RaceNumber: 88}, 0) != "NORRIS" {
		t.Fatal("name expansion changed custom identity")
	}
	a := analysisFixture(15)
	a.data = t.TempDir()

	table := analysisTable(a.analysisView(), "analysis-results")
	titles := []string{}
	for _, col := range table["columns"].([]M) {
		titles = append(titles, col["title"].(string))
	}
	if !reflect.DeepEqual(titles, []string{"POS", "Driver full name", "Team", "Compounds used", "Gap", "Grid", "+/-"}) {
		t.Fatal(titles)
	}
	if cell, ok := a.podiumPosition(1).(M); !ok || cell["text"] != "🥇" || cell["sort"] != 1 || cell["image"] != nil {
		t.Fatal("native podium medal not emitted")
	}
}
