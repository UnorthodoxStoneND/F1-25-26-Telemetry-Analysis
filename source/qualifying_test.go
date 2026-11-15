package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func qualifyingFixture(kinds []int) *QualifyingWeekend {
	q := &QualifyingWeekend{}
	for stage, kind := range kinds {
		meta := RecordingMeta{ID: strings.Repeat(string(rune('a'+stage)), 32), Session: kind, State: "completed", Profile: "Career", Season: 42, Weekend: 7, Track: 1, Started: time.Unix(1000+int64(stage)*1000, 0)}
		m := newModel(meta)
		for i := 0; i < 6; i++ {
			d := &m.Drivers[i]
			d.Name = []string{"ALPHA", "BRAVO", "CHARLIE", "DELTA", "ECHO", "FOXTROT"}[i]
			d.RaceNumber = i + 1
			d.TeamKnown = true
			d.TeamID = i / 2
			if i < 6-stage*2 {
				ms := uint32(90000 - stage*1000 + i*100)
				d.Best = ms
				d.Laps[1] = &RecordedLap{Time: ms, Valid: true}
				d.Completed = 1
				d.Position = i + 1
				d.Classified = true
			}
		}
		q.Stages = append(q.Stages, QualifyingStage{Kind: kind, Meta: meta, Model: m})
	}
	q.Anchor = q.Stages[len(q.Stages)-1].Meta.ID
	return q
}
func TestQualifyingStageLinksAndRestartBoundaries(t *testing.T) {
	q := qualifyingFixture([]int{5, 6, 7})
	items := []RecordingMeta{}
	for _, s := range q.Stages {
		items = append(items, s.Meta)
	}
	other := items[0]
	other.ID = strings.Repeat("d", 32)
	other.Session = 10
	items = append(items, other)
	restart := items[0]
	restart.ID = strings.Repeat("e", 32)
	restart.Started = time.Unix(5000, 0)
	items = append(items, restart)
	linked := qualifyingMetas(q.Stages[2].Meta, items)
	if len(linked) != 3 || linked[5].ID != q.Stages[0].Meta.ID {
		t.Fatal("mixed sprint/normal or restarted stages", linked)
	}
	if got := qualifyingMetas(restart, items); len(got) != 1 {
		t.Fatal("new attempt borrowed old stages", got)
	}
	unknown := q.Stages[2].Meta
	unknown.Weekend = 0
	if len(qualifyingMetas(unknown, items)) != 1 {
		t.Fatal("unknown weekend was linked")
	}
	foreign := items[0]
	foreign.ID = strings.Repeat("f", 32)
	foreign.Profile = "Different"
	foreign.Started = time.Unix(2900, 0)
	items = append(items, foreign)
	if qualifyingMetas(q.Stages[2].Meta, items)[5].ID != q.Stages[0].Meta.ID {
		t.Fatal("cross-profile link")
	}
}
func TestQualifyingAdvancementAndCrossStageTeammates(t *testing.T) {
	q := qualifyingFixture([]int{10, 11, 12})
	for i, s := range q.Stages {
		if len(qualifyingRows(s)) != 6-i*2 {
			t.Fatal("inactive prior-stage driver included", i)
		}
	}
	key := qualifyingRows(q.Stages[0])[5].Key
	if q.advancement(0, key) != -1 || q.advancement(0, qualifyingRows(q.Stages[0])[0].Key) != 1 {
		t.Fatal("recorded knockout transition")
	}
	q.Stages[1].Meta.State = "recording"
	if q.advancement(0, key) != 0 {
		t.Fatal("unfinished next stage proves elimination")
	}
	q.Stages[1].Meta.State = "completed"
	q.Stages[0].Model.Drivers[1].Best = 79000
	q.Stages[0].Model.Drivers[1].Laps[1].Time = 79000
	q.Stages[2].Model.Drivers[0].Best = 80000
	q.Stages[2].Model.Drivers[0].Laps[1].Time = 80000
	duels := q.teammateDuels()
	found := false
	for _, d := range duels {
		if strings.HasPrefix(d.First, "BRA") {
			found = true
			if d.Gap != 1000 || d.Stage != "SQ1 / SQ3" {
				t.Fatal("not each driver's fastest lap across stages", d)
			}
		}
	}
	if !found {
		t.Fatal("cross-stage teammate comparison missing", duels)
	}
	q.Stages[1].Model = nil
	if q.advancement(0, key) != 0 {
		t.Fatal("missing next stage proves elimination")
	}
}
func TestQualifyingAdvancedView(t *testing.T) {
	for _, kinds := range [][]int{{5, 6, 7}, {10, 11, 12}} {
		q := qualifyingFixture(kinds)
		m := q.Stages[2].Model
		a := App{data: t.TempDir(), rec: &Recorder{items: []RecordingMeta{m.Meta}}, analysis: AnalysisSelection{Model: m, Qualifying: q}}
		a.displayCache = filepath.Join(a.data, "display")
		view := a.analysisView()
		tabs := wireNodes(view, "tabs")[0]["items"].([]any)
		if tabs[1].(map[string]any)["label"] != "Advanced Telemetry" || len(wireNodes(view, "card")) != 3 {
			t.Fatal("qualifying stage results layout")
		}
		a.analysis.Tab = "advanced"
		view = a.analysisView()
		if len(wireNodes(view, "image")) != 4 {
			t.Fatal("three advanced charts missing")
		}
		raw, _ := json.Marshal(M{"event": M{"submit": M{"form": "analysis-qual-stage-" + m.Meta.ID, "values": M{"stage": fmt.Sprint(kinds[0])}}}})
		if _, e := a.analysisEvent(raw); e != nil || a.analysis.QualStage != kinds[0] {
			t.Fatal("stage selector", e)
		}
		a.analysis.Tab = "laps"
		a.analysisView()
		if a.analysis.Tab != "stats" {
			t.Fatal("Laps & Sectors still accessible")
		}
	}
	for _, kind := range []int{8, 9, 13, 14} {
		a := analysisFixture(kind)
		a.data = t.TempDir()
		a.displayCache = filepath.Join(a.data, "display")
		table := analysisTable(a.analysisView(), "analysis-results")
		if table == nil || len(table["columns"].([]M)) != 8 {
			t.Fatal("single-stage results differ from practice", kind)
		}
		a.analysis.Tab = "advanced"
		if len(wireNodes(a.analysisView(), "image")) != 3 {
			t.Fatal("single-stage advanced charts", kind)
		}
	}
}
func TestQualifyingPreviewFromEnvironment(t *testing.T) {
	out := os.Getenv("UDP_TELEMETRY_QUAL_PREVIEW")
	if out == "" {
		t.Skip("no qualifying preview configured")
	}
	q := qualifyingFixture([]int{5, 6, 7})
	m := q.Stages[2].Model
	if path := os.Getenv("UDP_TELEMETRY_QUAL_META"); path != "" {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var meta RecordingMeta
		if e = json.Unmarshal(b, &meta); e != nil {
			t.Fatal(e)
		}
		m, e = loadAnalysis(filepath.Dir(path), meta)
		if e != nil {
			t.Fatal(e)
		}
		q = loadQualifyingWeekend(filepath.Dir(path), meta, []RecordingMeta{meta}, m)
	}
	a := App{data: out, displayCache: filepath.Join(out, "display"), rec: &Recorder{items: []RecordingMeta{m.Meta}}, analysis: AnalysisSelection{Model: m, Qualifying: q}}
	for _, tab := range []string{"stats", "advanced"} {
		a.analysis.Tab = tab
		view := a.analysisView()
		b, _ := json.MarshalIndent(view, "", "  ")
		_ = os.MkdirAll(out, 0755)
		if e := os.WriteFile(filepath.Join(out, tab+".json"), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	if len(q.Stages) > 1 && q.Stages[0].Model != nil {
		a.analysis.Tab = "advanced"
		a.analysis.QualStage = q.Stages[0].Kind
		a.analysis.QualStageSession = m.Meta.ID
		b, _ := json.MarshalIndent(a.analysisView(), "", "  ")
		if e := os.WriteFile(filepath.Join(out, "advanced-first.json"), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	if !reflect.DeepEqual(qualifyingKinds(10), []int{10, 11, 12}) {
		t.Fatal("sprint qualifying kinds")
	}
}
