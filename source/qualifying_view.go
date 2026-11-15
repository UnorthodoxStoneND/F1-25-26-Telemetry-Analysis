package main

import (
	"fmt"
	"sort"
)

func (a *App) qualifyingBlocks(m *AnalysisModel, advanced bool) []M {
	q := a.currentQualifying(m)
	if !advanced {
		if len(q.Stages) == 1 {
			rows := qualifyingRows(q.Stages[0])
			ordered := []int{}
			for _, r := range rows {
				ordered = append(ordered, r.Driver)
			}
			sort.SliceStable(ordered, func(i, j int) bool {
				a, b := m.Drivers[ordered[i]].Position, m.Drivers[ordered[j]].Position
				if a <= 0 {
					a = 999
				}
				if b <= 0 {
					b = 999
				}
				return a < b
			})
			return []M{a.practiceResults(m, ordered)}
		}
		cards := []M{}
		for index := len(q.Stages) - 1; index >= 0; index-- {
			s := q.Stages[index]
			content := []M{}
			if s.Model == nil {
				message := "Not recorded"
				if s.Error != "" {
					message = "Recording unavailable: " + s.Error
				}
				content = append(content, textBlock(message))
			} else {
				rows := qualifyingRows(s)
				advancedRows, outRows := []QualifyingRow{}, []QualifyingRow{}
				for _, r := range rows {
					if q.advancement(index, r.Key) == -1 {
						outRows = append(outRows, r)
					} else {
						advancedRows = append(advancedRows, r)
					}
				}
				best := uint32(0)
				for _, r := range rows {
					if r.Best > 0 && (best == 0 || r.Best < best) {
						best = r.Best
					}
				}
				if len(advancedRows) > 0 {
					content = append(content, a.qualifyingTable(s, advancedRows, best, ""))
				}
				if len(outRows) > 0 {
					content = append(content, M{"text": "Knocked out", "tone": "bad"}, a.qualifyingTable(s, outRows, best, "-out"))
				}
				if len(rows) == 0 {
					content = append(content, textBlock("No driver timing recorded"))
				}
			}
			cards = append(cards, raceCard(sessionLabel(s.Kind), content...))
		}
		return []M{{"grid": M{"columns": 3, "blocks": cards}}}
	}
	if a.analysis.QualStageSession != m.Meta.ID {
		a.analysis.QualStageSession = m.Meta.ID
		a.analysis.QualStage = m.Meta.Session
	}
	selected := -1
	options := []M{}
	for index, s := range q.Stages {
		if s.Model != nil {
			options = append(options, M{"value": fmt.Sprint(s.Kind), "label": sessionLabel(s.Kind)})
			if s.Kind == a.analysis.QualStage {
				selected = index
			}
		}
	}
	if selected < 0 {
		for index, s := range q.Stages {
			if s.Model != nil {
				selected = index
				a.analysis.QualStage = s.Kind
				break
			}
		}
	}
	if selected < 0 {
		return []M{textBlock("No qualifying data recorded")}
	}
	out := []M{}
	if len(options) > 1 {
		out = append(out, M{"form": M{"id": "analysis-qual-stage-" + m.Meta.ID, "live": true, "fields": []M{selectionField("stage", "Qualifying stage", fmt.Sprint(a.analysis.QualStage), options)}}})
	}
	signature := q.signature()
	out = append(out, a.raceImage(m, fmt.Sprintf("quali-timeline/%d/%s", selected, signature), func(path string) error { return renderQualifyingTimeline(path, q, selected) }))
	if len(q.Stages) > 1 {
		out = append(out, a.raceImage(m, "quali-knockout/"+signature, func(path string) error { return renderKnockoutProgression(path, q) }))
	}
	out = append(out, a.raceImage(m, "quali-teammates/"+signature, func(path string) error { return renderTeammateDuels(path, q) }))
	return out
}
func (a *App) qualifyingTable(s QualifyingStage, rows []QualifyingRow, leader uint32, suffix string) M {
	values := [][]any{}
	for _, r := range rows {
		d := s.Model.Drivers[r.Driver]
		gap := "—"
		if r.Best > 0 && leader > 0 && r.Best > leader {
			gap = fmt.Sprintf("+%.3f", float64(r.Best-leader)/1000)
		}
		values = append(values, []any{a.podiumPosition(r.Rank), resultDriverName(d, r.Driver), s.Model.teamName(d), lapTime(r.Best), gap})
	}
	table := tableBlock(fmt.Sprintf("qualifying-results-%d%s", s.Kind, suffix), []string{"POS", "Driver", "Team", "Time", "Gap"}, values)
	for i, w := range []int{34, 130, 130, 84, 78} {
		table["table"].(M)["columns"].([]M)[i]["width"] = w
	}
	return table
}
