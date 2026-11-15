package main

import (
	"fmt"
	"sort"
	"strings"
)

func qualifyingSession(kind int) bool { return kind >= 5 && kind <= 14 }
func qualifyingKinds(kind int) []int {
	if kind >= 5 && kind <= 7 {
		return []int{5, 6, 7}
	}
	if kind >= 10 && kind <= 12 {
		return []int{10, 11, 12}
	}
	return []int{kind}
}

type QualifyingStage struct {
	Kind  int
	Meta  RecordingMeta
	Model *AnalysisModel
	Error string
}
type QualifyingWeekend struct {
	Stages []QualifyingStage
	Anchor string
}

func qualifyingMetas(anchor RecordingMeta, catalog []RecordingMeta) map[int]RecordingMeta {
	out := map[int]RecordingMeta{anchor.Session: anchor}
	kinds := qualifyingKinds(anchor.Session)
	if len(kinds) == 1 || anchor.Season == 0 || anchor.Weekend == 0 {
		return out
	}
	allowed := map[int]bool{}
	for _, k := range kinds {
		allowed[k] = true
	}
	items := []RecordingMeta{}
	for _, m := range catalog {
		if m.Profile == anchor.Profile && m.Season == anchor.Season && m.Weekend == anchor.Weekend && m.Track == anchor.Track && allowed[m.Session] {
			items = append(items, m)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Started.Equal(items[j].Started) {
			return items[i].ID < items[j].ID
		}
		return items[i].Started.Before(items[j].Started)
	})
	at := -1
	for i, m := range items {
		if m.ID == anchor.ID {
			at = i
			break
		}
	}
	if at < 0 {
		return out
	}
	// A new Q1/SQ1 starts another attempt. Never borrow later stages across it.
	start, end := 0, len(items)
	for i := 0; i <= at; i++ {
		if items[i].Session == kinds[0] {
			start = i
		}
	}
	for i := at + 1; i < len(items); i++ {
		if items[i].Session <= anchor.Session {
			end = i
			break
		}
	}
	for _, m := range items[start:end] {
		if m.Session != anchor.Session && !(m.Session > anchor.Session && m.Started.Before(anchor.Started)) {
			out[m.Session] = m
		}
	}
	out[anchor.Session] = anchor
	return out
}
func loadQualifyingWeekend(dir string, meta RecordingMeta, catalog []RecordingMeta, current *AnalysisModel) *QualifyingWeekend {
	q := &QualifyingWeekend{Anchor: meta.ID}
	metas := qualifyingMetas(meta, catalog)
	for _, kind := range qualifyingKinds(meta.Session) {
		stage := QualifyingStage{Kind: kind}
		if entry, ok := metas[kind]; ok {
			stage.Meta = entry
			if entry.ID == meta.ID {
				stage.Model = current
			} else {
				m, e := loadAnalysis(dir, entry)
				if e != nil {
					stage.Error = e.Error()
				} else {
					// Peer stages need timing/identity only; release driving traces.
					compact := newModel(m.Meta)
					compact.Frames = m.Frames
					for i, d := range m.Drivers {
						d.Samples = nil
						d.raceObservations = nil
						d.pitPoints = nil
						compact.Drivers[i] = d
					}
					stage.Model = compact
				}
			}
		}
		q.Stages = append(q.Stages, stage)
	}
	return q
}
func (a *App) currentQualifying(m *AnalysisModel) *QualifyingWeekend {
	if q := a.analysis.Qualifying; q != nil && q.Anchor == m.Meta.ID {
		return q
	}
	q := &QualifyingWeekend{Anchor: m.Meta.ID}
	for _, kind := range qualifyingKinds(m.Meta.Session) {
		s := QualifyingStage{Kind: kind}
		if kind == m.Meta.Session {
			s.Meta = m.Meta
			s.Model = m
		}
		q.Stages = append(q.Stages, s)
	}
	return q
}
func (q *QualifyingWeekend) signature() string {
	values := []string{}
	for _, s := range q.Stages {
		frames := 0
		if s.Model != nil {
			frames = s.Model.Frames
		}
		values = append(values, fmt.Sprintf("%d:%s:%d:%s", s.Kind, s.Meta.ID, frames, s.Meta.State))
	}
	return strings.Join(values, "/")
}

type QualifyingRow struct {
	Key    string
	Driver int
	Rank   int
	Best   uint32
}

func qualDriverKey(d RecordedDriver, index int) string {
	return fmt.Sprintf("%d:%s", d.RaceNumber, strings.ToUpper(strings.TrimSpace(d.name(index))))
}
func qualTeamKey(d RecordedDriver, index int) string {
	if !d.TeamKnown {
		return "unknown:" + qualDriverKey(d, index)
	}
	return fmt.Sprintf("%t:%d", d.MyTeam, d.TeamID)
}
func qualifyingRows(s QualifyingStage) []QualifyingRow {
	if s.Model == nil {
		return nil
	}
	rows := []QualifyingRow{}
	for _, i := range raceDriverIndices(s.Model) {
		d := s.Model.Drivers[i]
		best, _, laps := practiceTimes(&d)
		first := s.Kind == 5 || s.Kind == 10 || len(qualifyingKinds(s.Kind)) == 1
		if !first && best == 0 && laps == 0 {
			continue
		}
		rows = append(rows, QualifyingRow{Key: qualDriverKey(d, i), Driver: i, Best: best})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Best != b.Best {
			if a.Best == 0 {
				return false
			}
			if b.Best == 0 {
				return true
			}
			return a.Best < b.Best
		}
		pa, pb := s.Model.Drivers[a.Driver].Position, s.Model.Drivers[b.Driver].Position
		if pa == 0 {
			pa = 999
		}
		if pb == 0 {
			pb = 999
		}
		if pa != pb {
			return pa < pb
		}
		return a.Key < b.Key
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows
}

// 1 = observed advance, -1 = confirmed absent from completed next stage,
// 2 = final completed stage; 0 means the result is not yet established.
func (q *QualifyingWeekend) advancement(stage int, key string) int {
	if stage+1 >= len(q.Stages) {
		if q.Stages[stage].Meta.State == "completed" {
			return 2
		}
		return 0
	}
	next := q.Stages[stage+1]
	if next.Model == nil {
		return 0
	}
	rows := qualifyingRows(next)
	for _, r := range rows {
		if r.Key == key {
			return 1
		}
	}
	if q.Stages[stage].Meta.State == "completed" && next.Meta.State == "completed" && len(rows) > 0 {
		return -1
	}
	return 0
}

type qualVisual struct {
	Driver int
	Dashed bool
}

func (q *QualifyingWeekend) visuals() map[string]qualVisual {
	teams := map[string]map[string]int{}
	for _, s := range q.Stages {
		for _, r := range qualifyingRows(s) {
			d := s.Model.Drivers[r.Driver]
			team := qualTeamKey(d, r.Driver)
			if teams[team] == nil {
				teams[team] = map[string]int{}
			}
			if _, ok := teams[team][r.Key]; !ok {
				teams[team][r.Key] = r.Driver
			}
		}
	}
	out := map[string]qualVisual{}
	for _, drivers := range teams {
		keys := []string{}
		rep := 24
		for key, i := range drivers {
			keys = append(keys, key)
			rep = min(rep, i)
		}
		sort.Strings(keys)
		for n, key := range keys {
			out[key] = qualVisual{rep, n%2 == 1}
		}
	}
	return out
}

type TeammateDuel struct {
	First, Second string
	Gap           uint32
	Stage         string
	Driver        int
}

func (q *QualifyingWeekend) teammateDuels() []TeammateDuel {
	visuals := q.visuals()
	type bestDriver struct {
		Row   QualifyingRow
		Model *AnalysisModel
		Stage string
	}
	groups := map[string]map[string]bestDriver{}
	for _, s := range q.Stages {
		for _, r := range qualifyingRows(s) {
			if r.Best == 0 {
				continue
			}
			d := s.Model.Drivers[r.Driver]
			if !d.TeamKnown {
				continue
			}
			team := qualTeamKey(d, r.Driver)
			if groups[team] == nil {
				groups[team] = map[string]bestDriver{}
			}
			previous, ok := groups[team][r.Key]
			if !ok || r.Best < previous.Row.Best {
				groups[team][r.Key] = bestDriver{r, s.Model, sessionLabel(s.Kind)}
			}
		}
	}
	out := []TeammateDuel{}
	for _, drivers := range groups {
		if len(drivers) != 2 {
			continue
		}
		pair := []bestDriver{}
		for _, d := range drivers {
			pair = append(pair, d)
		}
		sort.Slice(pair, func(i, j int) bool {
			if pair[i].Row.Best == pair[j].Row.Best {
				return pair[i].Row.Key < pair[j].Row.Key
			}
			return pair[i].Row.Best < pair[j].Row.Best
		})
		a, b := pair[0], pair[1]
		stage := a.Stage
		if a.Stage != b.Stage {
			stage += " / " + b.Stage
		}
		out = append(out, TeammateDuel{driverCode(a.Model, a.Row.Driver), driverCode(b.Model, b.Row.Driver), b.Row.Best - a.Row.Best, stage, visuals[a.Row.Key].Driver})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Gap == out[j].Gap {
			return out[i].First < out[j].First
		}
		return out[i].Gap < out[j].Gap
	})
	return out
}
