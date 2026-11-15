package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type AnalysisSelection struct {
	Save, Season, Weekend, Session, Tab                string
	Driver, Lap, ReferenceDriver, ReferenceLap, Window int
	Model                                              *AnalysisModel
	Loading, Error                                     string
	Generation                                         int
	Compare                                            bool
	EditSave                                           bool
	RaceDrivers                                        uint32
	RaceSelectionSession                               string
	RaceLap                                            int
	Qualifying                                         *QualifyingWeekend
	QualStage                                          int
	QualStageSession                                   string
}

func sessionLabel(n int) string {
	names := []string{"Session", "FP1", "FP2", "FP3", "Short practice", "Q1", "Q2", "Q3", "Short qualifying", "One-shot qualifying", "SQ1", "SQ2", "SQ3", "Short sprint qualifying", "One-shot sprint qualifying", "Race", "Race 2", "Race 3", "Time Trial"}
	if n >= 0 && n < len(names) {
		return names[n]
	}
	return "Session"
}
func trackLabel(n int) string {
	names := map[int]string{0: "Melbourne", 1: "Paul Ricard", 2: "Shanghai", 3: "Sakhir", 4: "Barcelona", 5: "Monaco", 6: "Montreal", 7: "Silverstone", 8: "Hockenheim", 9: "Hungaroring", 10: "Spa", 11: "Monza", 12: "Singapore", 13: "Suzuka", 14: "Abu Dhabi", 15: "Texas", 16: "Brazil", 17: "Austria", 18: "Sochi", 19: "Mexico", 20: "Baku", 26: "Zandvoort", 27: "Imola", 29: "Jeddah", 30: "Miami", 31: "Las Vegas", 32: "Lusail", 39: "Silverstone reverse", 40: "Austria reverse", 41: "Zandvoort reverse", 42: "Madrid"}
	if s := names[n]; s != "" {
		return s
	}
	return fmt.Sprintf("Track %d", n)
}
func seasonKey(m RecordingMeta) string {
	if m.Season == 0 {
		return "unknown"
	}
	return strconv.FormatUint(uint64(m.Season), 10)
}
func weekendKey(m RecordingMeta) string {
	if m.Weekend == 0 {
		return "session-" + m.ID
	}
	return strconv.FormatUint(uint64(m.Weekend), 10)
}
func selectionField(id, label, value string, options []M) M {
	return M{"select": M{"id": id, "label": label, "value": value, "options": options, "search": true}}
}
func tableBlock(id string, titles []string, rows [][]any) M {
	cols := []M{}
	for i, t := range titles {
		cols = append(cols, M{"key": fmt.Sprint(i), "title": t})
	}
	// Native Cell is string | CellSpec; bare JSON numbers are rejected by
	// Swofty's deserializer even though ordinary JSON accepts them.
	for _, row := range rows {
		for i, value := range row {
			switch value.(type) {
			case string, M:
			case nil:
				row[i] = "—"
			default:
				row[i] = fmt.Sprint(value)
			}
		}
	}
	return M{"table": M{"id": id, "columns": cols, "rows": rows, "compact": true, "empty": "No recorded data available."}}
}

func validLapOptions(d *RecordedDriver) []M {
	opts := []M{}
	for _, n := range d.laps() {
		l := d.Laps[n]
		if l.Valid && l.Time > 0 {
			opts = append(opts, M{"value": fmt.Sprint(n), "label": fmt.Sprintf("Lap %d · %s", n, lapTime(l.Time))})
		}
	}
	return opts
}
func selectLap(current *int, opts []M) {
	for _, o := range opts {
		if o["value"] == fmt.Sprint(*current) {
			return
		}
	}
	*current = 0
	if len(opts) > 0 {
		*current, _ = strconv.Atoi(opts[len(opts)-1]["value"].(string))
	}
}
func comparisonFields(s *AnalysisSelection, drivers, refOpts []M) []M {
	ropts := append([]M{{"value": "0", "label": "No comparison"}}, refOpts...)
	if s.Compare && s.ReferenceLap < 0 {
		s.ReferenceLap = 0
		for _, o := range refOpts {
			n, _ := strconv.Atoi(o["value"].(string))
			s.ReferenceLap = n
			if n == s.Lap {
				break
			}
		}
	}
	found := false
	for _, o := range refOpts {
		if o["value"] == fmt.Sprint(s.ReferenceLap) {
			found = true
		}
	}
	if !s.Compare || !found {
		s.Compare = false
		s.ReferenceLap = 0
	}
	value := "none"
	if s.Compare {
		value = fmt.Sprint(s.ReferenceDriver)
	}
	fields := []M{selectionField("reference_driver", "Compare driver", value, append([]M{{"value": "none", "label": "No comparison"}}, drivers...))}
	if s.Compare {
		fields = append(fields, selectionField("reference_lap", "Compare lap", fmt.Sprint(s.ReferenceLap), ropts))
	}
	return fields
}
func lapDelta(selected, reference uint32) M {
	if selected == 0 || reference == 0 {
		return M{"text": "—", "tone": "muted"}
	}
	delta := int64(selected) - int64(reference)
	tone := "neutral"
	if delta < 0 {
		tone = "good"
	}
	if delta > 0 {
		tone = "bad"
	}
	return M{"text": fmt.Sprintf("%+.3f s", float64(delta)/1000), "tone": tone}
}
func (a *App) requestAnalysis(meta RecordingMeta, refresh bool) {
	s := &a.analysis
	if !refresh && ((s.Model != nil && s.Model.Meta.ID == meta.ID) || s.Loading == meta.ID) {
		return
	}
	if a.rec.current != nil && a.rec.current.ID == meta.ID {
		if e := a.rec.flush(); e != nil {
			s.Error = e.Error()
			return
		}
	}
	s.Generation++
	generation := s.Generation
	s.Loading = meta.ID
	s.Error = ""
	s.Model = nil
	s.Qualifying = nil
	dir := a.rec.dir
	catalog := a.rec.catalog()
	go func() {
		m, e := loadAnalysis(dir, meta)
		var qualifying *QualifyingWeekend
		if e == nil && qualifyingSession(meta.Session) {
			qualifying = loadQualifyingWeekend(dir, meta, catalog, m)
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.analysis.Generation != generation {
			return
		}
		a.analysis.Loading = ""
		if e != nil {
			a.analysis.Error = e.Error()
		} else {
			a.analysis.Model = m
			a.analysis.Qualifying = qualifying
		}
		a.send(M{"method": "invalidate", "params": M{"section": "analysis"}})
	}()
}
func (a *App) analysisView() M {
	a.mu.Lock()
	defer a.mu.Unlock()
	blocks := []M{}
	s := &a.analysis
	finish := func() M {
		return M{"subtitle": "Saved UDP sessions · v" + addonVersion, "tick": true, "body": M{"blocks": blocks}}
	}
	if a.rec == nil {
		blocks = append(blocks, textBlock("Recording unavailable: "+a.recordingErr))
		return finish()
	}
	if a.rec.err != "" {
		blocks = append(blocks, textBlock("Recording error: "+a.rec.err))
	}
	items := a.rec.catalog()
	if len(items) == 0 {
		blocks = append(blocks, textBlock("No saved telemetry yet. Sessions are recorded automatically while this addon is enabled and receiving UDP."))
		return finish()
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Started.After(items[j].Started) })
	// Resolve hierarchical filters in order, defaulting to the newest matching
	// recording. Unknown link identifiers are kept separate, never guessed.
	fields := []M{}
	filtered := items
	keys := []string{"save", "season", "weekend", "session"}
	labels := []string{"Save", "Season", "Race weekend", "Session"}
	values := []*string{&s.Save, &s.Season, &s.Weekend, &s.Session}
	previousSession := s.Session
	for stage := 0; stage < 4; stage++ {
		opts := []M{}
		seen := map[string]bool{}
		valid := false
		valueFor := func(m RecordingMeta) string {
			switch stage {
			case 0:
				return m.Save
			case 1:
				return seasonKey(m)
			case 2:
				return weekendKey(m)
			}
			return m.ID
		}
		for _, m := range filtered {
			v := valueFor(m)
			if seen[v] {
				continue
			}
			seen[v] = true
			label := v
			switch stage {
			case 1:
				if v == "unknown" {
					label = "Unknown season"
				} else {
					label = "Season ID " + v
				}
			case 2:
				label = trackLabel(m.Track)
				if m.Weekend != 0 {
					label += fmt.Sprintf(" · %d", m.Weekend)
				} else {
					label += " · " + m.Started.Local().Format("02 Jan 15:04")
				}
			case 3:
				label = fmt.Sprintf("%s · %s · %s", recordingSessionLabel(m), m.Started.Local().Format("02 Jan 15:04:05"), m.State)
			}
			opts = append(opts, M{"value": v, "label": label})
			if *values[stage] == v {
				valid = true
			}
		}
		if !valid {
			*values[stage] = valueFor(filtered[0])
		}
		// A one-choice season adds no decision but forces a fourth field onto
		// another row in the launcher's three-column form layout.
		if stage != 1 || len(opts) > 1 {
			fields = append(fields, selectionField(keys[stage], labels[stage], *values[stage], opts))
		}
		next := []RecordingMeta{}
		for _, m := range filtered {
			if valueFor(m) == *values[stage] {
				next = append(next, m)
			}
		}
		filtered = next
	}
	blocks = append(blocks, M{"form": M{"id": "analysis-filters", "live": true, "columns": 3, "fields": fields}})
	meta := filtered[0]
	if previousSession != meta.ID {
		s.EditSave = false
		if previousSession != "" {
			s.Tab = "stats"
		}
	}
	a.requestAnalysis(meta, false)
	editLabel := "Edit save name"
	if s.EditSave {
		editLabel = "Cancel editing"
	}
	blocks = append(blocks, M{"buttons": []M{{"id": "analysis-refresh", "label": "Refresh", "icon": "refresh"}, {"id": "analysis-edit-save", "label": editLabel}}})
	if s.EditSave {
		blocks = append(blocks, M{"form": M{"id": "analysis-save", "apply": "Save name", "fields": []M{{"text": M{"id": "name", "label": "Save name", "value": meta.Save, "max_len": 80}}}}})
	}
	if s.Error != "" {
		blocks = append(blocks, textBlock(s.Error))
		return finish()
	}
	if s.Loading != "" || s.Model == nil {
		blocks = append(blocks, textBlock("Loading saved session…"))
		return finish()
	}
	m := s.Model
	blocks = append(blocks, a.summaryBlock(m))
	if m.Truncated && meta.State != "recording" {
		blocks = append(blocks, textBlock("Recovered up to the last complete UDP packet. The recording ended without a complete journal footer."))
	}
	tabs := []M{}
	timedOnly := meta.Session >= 1 && meta.Session <= 14
	isPractice := practiceSession(meta.Session)
	isQualifying := qualifyingSession(meta.Session)
	isRace := raceSession(meta.Session)
	if timedOnly && s.Tab != "laps" && s.Tab != "stats" && !(isPractice && s.Tab == "practice") && !(isQualifying && s.Tab == "advanced") {
		s.Tab = "stats"
	}
	ids := []string{"stats", "traces", "laps", "tyres", "positions"}
	if isPractice {
		ids = []string{"stats", "practice"}
		if s.Tab == "laps" {
			s.Tab = "stats"
		}
	}
	if isQualifying {
		ids = []string{"stats", "advanced"}
		if s.Tab == "laps" {
			s.Tab = "stats"
		}
	}
	if isRace {
		ids = []string{"stats", "traces", "tyres", "positions", "progression"}
		if s.Tab == "laps" {
			s.Tab = "stats"
		}
	}
	for _, v := range ids {
		if timedOnly && v != "laps" && v != "stats" && !(isPractice && v == "practice") && !(isQualifying && v == "advanced") {
			continue
		}
		label := map[string]string{"traces": "Traces", "laps": "Laps & sectors", "tyres": "Tyres & pits", "positions": "Positions", "stats": "Session stats"}[v]
		if v == "practice" {
			label = "Session Analysis"
		}
		if v == "advanced" {
			label = "Advanced Telemetry"
		}
		if isRace {
			label = map[string]string{"stats": "Overview", "traces": "Lap Analysis", "tyres": "Pit Stops", "positions": "Race Strategy", "progression": "Race Progression"}[v]
		}
		if v == "stats" {
			label = resultTabLabel(meta.Session)
		}
		tabs = append(tabs, M{"id": v, "label": label})
	}
	if s.Tab == "" {
		s.Tab = "stats"
	}
	blocks = append(blocks, M{"tabs": M{"id": "analysis-tabs", "active": s.Tab, "items": tabs}})
	if isQualifying {
		blocks = append(blocks, a.qualifyingBlocks(m, s.Tab == "advanced")...)
		return finish()
	}
	if isPractice && s.Tab == "practice" {
		blocks = append(blocks, a.practiceAnalysisBlocks(m)...)
		return finish()
	}
	if isRace && s.Tab != "stats" {
		blocks = append(blocks, a.raceAnalysisBlocks(m)...)
		return finish()
	}
	drivers := []M{}
	available := map[int]bool{}
	for i, d := range m.Drivers {
		if d.Name != "" || len(d.Samples) > 0 || len(d.Laps) > 0 {
			drivers = append(drivers, M{"value": fmt.Sprint(i), "label": d.name(i)})
			available[i] = true
		}
	}
	if len(drivers) == 0 {
		blocks = append(blocks, textBlock("Waiting for driver or lap packets in this recording."))
		return finish()
	}
	if !available[s.Driver] {
		s.Driver = meta.Player
		if !available[s.Driver] {
			for i := 0; i < 24; i++ {
				if available[i] {
					s.Driver = i
					break
				}
			}
		}
	}
	if !available[s.ReferenceDriver] {
		s.ReferenceDriver = s.Driver
	}
	d := &m.Drivers[s.Driver]
	controls := []M{selectionField("driver", "Driver", fmt.Sprint(s.Driver), drivers)}
	if s.Tab == "traces" {
		lapOpts := func(d *RecordedDriver) []M {
			opts := []M{}
			for _, n := range d.laps() {
				if len(d.trace(n)) > 0 {
					label := fmt.Sprintf("Lap %d · %s", n, lapTime(d.Laps[n].Time))
					if !d.Laps[n].Valid {
						label += " · unverified / invalid"
					}
					opts = append(opts, M{"value": fmt.Sprint(n), "label": label})
				}
			}
			return opts
		}
		opts := lapOpts(d)
		if len(opts) == 0 {
			blocks = append(blocks, M{"form": M{"id": "analysis-controls", "live": true, "fields": controls}}, textBlock("No paired lap-distance and car-telemetry packets were recorded for this driver."))
			return finish()
		}
		found := false
		for _, o := range opts {
			if o["value"] == fmt.Sprint(s.Lap) {
				found = true
			}
		}
		if !found {
			s.Lap, _ = strconv.Atoi(opts[len(opts)-1]["value"].(string))
		}
		ref := &m.Drivers[s.ReferenceDriver]
		controls = append(controls, selectionField("lap", "Lap", fmt.Sprint(s.Lap), opts))
		controls = append(controls, comparisonFields(s, drivers, lapOpts(ref))...)
		controls = append(controls, selectionField("window", "Distance range", fmt.Sprint(s.Window), []M{{"value": "0", "label": "Full lap"}, {"value": "1", "label": "First quarter"}, {"value": "2", "label": "Second quarter"}, {"value": "3", "label": "Third quarter"}, {"value": "4", "label": "Final quarter"}}))
		blocks = append(blocks, M{"form": M{"id": "analysis-controls", "live": true, "columns": 3, "fields": controls}})
		selected := d.trace(s.Lap)
		var comparison []TraceSample
		if s.Compare {
			comparison = ref.trace(s.ReferenceLap)
		} else {
			s.ReferenceLap = 0
		}
		if warning := traceCoverage(selected, d.Laps[s.Lap].Time); warning != "" {
			blocks = append(blocks, textBlock(d.name(s.Driver)+": "+warning+"."))
		}
		if warning := traceResolution(selected); warning != "" {
			blocks = append(blocks, textBlock(d.name(s.Driver)+": "+warning+"."))
		}
		if s.ReferenceLap > 0 {
			if warning := traceCoverage(comparison, ref.Laps[s.ReferenceLap].Time); warning != "" {
				blocks = append(blocks, textBlock(ref.name(s.ReferenceDriver)+": "+warning+"."))
			}
			if warning := traceResolution(comparison); warning != "" {
				blocks = append(blocks, textBlock(ref.name(s.ReferenceDriver)+": "+warning+"."))
			}
		}
		path := filepath.Join(a.data, fmt.Sprintf("trace-v8-%s-%d-%d-%d-%d-%d-%d.png", meta.ID, m.Frames, s.Driver, s.Lap, s.ReferenceDriver, s.ReferenceLap, s.Window))
		_, chartErr := os.Stat(path)
		if chartErr != nil {
			chartErr = renderTrace(path, selected, comparison, s.Window)
		}
		if chartErr == nil {
			path, chartErr = a.displayImage(path)
		}
		if e := chartErr; e != nil {
			blocks = append(blocks, textBlock(e.Error()))
		} else {
			caption := fmt.Sprintf("Blue: %s · lap %d", d.name(s.Driver), s.Lap)
			if s.ReferenceLap > 0 {
				caption += fmt.Sprintf(" | Orange: %s · lap %d", ref.name(s.ReferenceDriver), s.ReferenceLap)
			}
			blocks = append(blocks, M{"image": M{"path": path, "width": 1100, "height": 770, "fit": "contain", "caption": caption}})
		}
	} else if s.Tab == "laps" {
		opts := validLapOptions(d)
		selectLap(&s.Lap, opts)
		ref := &m.Drivers[s.ReferenceDriver]
		if s.Lap == 0 {
			s.Compare = false
		}
		comparisonControls := comparisonFields(s, drivers, validLapOptions(ref))
		if s.Compare {
			controls = append(controls, selectionField("lap", "Lap", fmt.Sprint(s.Lap), opts))
		}
		controls = append(controls, comparisonControls...)
		blocks = append(blocks, M{"form": M{"id": "analysis-controls", "live": true, "columns": 2, "fields": controls}})
		if s.Compare {
			selected, reference := d.Laps[s.Lap], ref.Laps[s.ReferenceLap]
			rows := [][]any{}
			for _, metric := range []struct {
				label string
				a, b  uint32
			}{{"Lap time", selected.Time, reference.Time}, {"Sector 1", selected.S1, reference.S1}, {"Sector 2", selected.S2, reference.S2}, {"Sector 3", selected.S3, reference.S3}} {
				rows = append(rows, []any{metric.label, lapTime(metric.a), lapTime(metric.b), lapDelta(metric.a, metric.b)})
			}
			blocks = append(blocks, tableBlock("analysis-lap-comparison", []string{"Metric", fmt.Sprintf("%s · lap %d", d.name(s.Driver), s.Lap), fmt.Sprintf("%s · lap %d", ref.name(s.ReferenceDriver), s.ReferenceLap), "Delta"}, rows))
		}
		rows := [][]any{}
		for _, n := range d.laps() {
			l := d.Laps[n]
			if !l.Valid || l.Time == 0 {
				continue
			}
			rows = append(rows, []any{n, lapTime(l.Time), lapTime(l.S1), lapTime(l.S2), lapTime(l.S3), knownInt(l.Position), compound(l.Compound), wearSummary(l.Wear)})
		}
		blocks = append(blocks, tableBlock("analysis-laps", []string{"Lap", "Time", "Sector 1", "Sector 2", "Sector 3", "Position", "Tyre", "Avg wear"}, rows))
	} else if s.Tab == "tyres" {
		blocks = append(blocks, M{"form": M{"id": "analysis-controls", "live": true, "fields": controls}})
		rows := [][]any{}
		for _, p := range d.Pits {
			state := "In progress / partial"
			if p.Complete {
				state = "Complete"
			}
			rows = append(rows, []any{p.Lap, compound(p.CompoundIn), compound(p.CompoundOut), durationMS(p.StopMS), durationMS(p.LaneMS), state})
		}
		blocks = append(blocks, tableBlock("analysis-pits", []string{"Lap", "Tyre in", "Tyre out", "Stationary", "Pit lane", "Capture"}, rows))
		rows = [][]any{}
		start := 1
		for i, st := range d.Stints {
			end := m.stintEndLabel(d, st, start)
			rows = append(rows, []any{i + 1, compound(st.Compound), start, end})
			if st.EndLap != 255 {
				start = st.EndLap + 1
			}
		}
		blocks = append(blocks, tableBlock("analysis-stints", []string{"Stint", "Compound", "Start lap", "End lap"}, rows))
	} else if s.Tab == "positions" {
		rows := [][]any{}
		for i, dr := range m.Drivers {
			for _, n := range dr.laps() {
				l := dr.Laps[n]
				if l.Position > 0 {
					rows = append(rows, []any{n, dr.name(i), l.Position})
				}
			}
		}
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i][0].(int) == rows[j][0].(int) {
				return rows[i][2].(int) < rows[j][2].(int)
			}
			return rows[i][0].(int) < rows[j][0].(int)
		})
		blocks = append(blocks, tableBlock("analysis-positions", []string{"Lap start", "Driver", "Position"}, rows))
	} else {
		fastest := uint32(0)
		who := "Unavailable"
		raceDuration := 0.0
		raceLaps := 0
		for i, dr := range m.Drivers {
			for _, l := range dr.Laps {
				if l.Valid && l.Time > 0 && (fastest == 0 || l.Time < fastest) {
					fastest = l.Time
					who = dr.name(i)
				}
			}
			if dr.Best > 0 && (fastest == 0 || dr.Best < fastest) {
				fastest = dr.Best
				who = dr.name(i)
			}
			if dr.Position == 1 && dr.RaceSeconds > 0 {
				raceDuration = dr.RaceSeconds
				raceLaps = dr.Completed
			}
		}
		for _, e := range m.Events {
			if e.Code == "FTLP" && e.A >= 0 && e.A < 24 && e.LapMS > 0 && (fastest == 0 || e.LapMS < fastest) {
				fastest = e.LapMS
				who = m.Drivers[e.A].name(e.A)
			}
		}
		blocks = append(blocks, facts(fact("Fastest lap", who+" · "+lapTime(fastest))))
		if meta.Session >= 15 && meta.Session <= 17 {
			duration := "Unavailable"
			if raceDuration > 0 {
				duration = clockDuration(raceDuration)
			}
			overtakes := 0
			for _, e := range m.Events {
				if e.Code == "OVTK" {
					overtakes++
				}
			}
			blocks = append(blocks, facts(fact("Race duration", duration), fact("Race laps", knownInt(raceLaps)), fact("Overtakes recorded", fmt.Sprint(overtakes)), fact("SC affected laps", fmt.Sprint(len(m.SCLaps))), fact("VSC affected laps", fmt.Sprint(len(m.VSCLaps)))), textBlock("Event and safety-car totals cover the captured portion only. A session joined late or a lost UDP packet may leave gaps."))
		}
		rows := [][]any{}
		ordered := []int{}
		for i, dr := range m.Drivers {
			if dr.Name != "" || len(dr.Laps) > 0 {
				ordered = append(ordered, i)
			}
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
		titles := []string{"Position", "Driver", "Team"}
		if !timedOnly {
			titles = append(titles, "Grid")
		}
		titles = append(titles, "Completed laps", "Best lap")
		unknownTeams := false
		for _, i := range ordered {
			dr := m.Drivers[i]
			teamName := m.teamName(dr)
			unknownTeams = unknownTeams || teamName == "—"
			row := []any{knownInt(dr.Position), dr.name(i), teamName}
			if !timedOnly {
				row = append(row, knownInt(dr.Grid))
			}
			rows = append(rows, append(row, knownInt(dr.Completed), lapTime(dr.Best)))
		}
		if raceSession(meta.Session) {
			titles = []string{"POS", "Driver full name", "Team", "Compounds used", "Gap", "Grid", "+/-"}
			rows = [][]any{}
			for _, i := range ordered {
				dr := m.Drivers[i]
				change := "—"
				if dr.Grid > 0 && dr.Position > 0 {
					change = fmt.Sprintf("%+d", dr.Grid-dr.Position)
				}
				rows = append(rows, []any{a.podiumPosition(dr.Position), resultDriverName(dr, i), m.teamName(dr), m.usedCompounds(i), m.resultGap(i), knownInt(dr.Grid), change})
			}
		}

		results := tableBlock("analysis-results", titles, rows)
		if isPractice {
			results = a.practiceResults(m, ordered)
		}
		if raceSession(meta.Session) {
			columns := results["table"].(M)["columns"].([]M)
			for i, width := range []int{60, 220, 400, 150, 140, 70, 70} {
				columns[i]["width"] = width
			}
		}
		blocks = append(blocks, results)
		if m.Meta.Teams == nil {
			message := "Team names were not captured for this recording."
			if m.Meta.TeamCaptureError != "" {
				message += " " + m.Meta.TeamCaptureError
			}
			blocks = append(blocks, textBlock(message))
		} else if unknownTeams {
			blocks = append(blocks, textBlock("Some team names were unavailable or could not be matched unambiguously."))
		}
	}
	return finish()
}
func knownInt(n int) string {
	if n <= 0 {
		return "—"
	}
	return fmt.Sprint(n)
}
func durationMS(n uint32) string {
	if n == 0 {
		return "—"
	}
	return fmt.Sprintf("%.3f s", float64(n)/1000)
}
func wearSummary(w *[4]float64) string {
	if w == nil {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", (w[0]+w[1]+w[2]+w[3])/4)
}
func (a *App) analysisEvent(raw json.RawMessage) (bool, error) {
	var p struct {
		Event struct {
			Click  struct{ ID string }
			Tab    struct{ ID, Tab string }
			Submit struct {
				Form    string
				Values  map[string]json.RawMessage
				Changed []string
			}
		}
	}
	if e := json.Unmarshal(raw, &p); e != nil {
		return false, e
	}
	if !strings.HasPrefix(p.Event.Click.ID, "analysis-") && !strings.HasPrefix(p.Event.Submit.Form, "analysis-") && p.Event.Tab.ID != "analysis-tabs" {
		return false, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s := &a.analysis
	if strings.HasPrefix(p.Event.Submit.Form, "analysis-qual-stage-") {
		if s.Model == nil || p.Event.Submit.Form != "analysis-qual-stage-"+s.Model.Meta.ID {
			return true, nil
		}
		var value string
		if json.Unmarshal(p.Event.Submit.Values["stage"], &value) != nil {
			return true, fmt.Errorf("invalid qualifying stage")
		}
		kind, e := strconv.Atoi(value)
		if e != nil {
			return true, fmt.Errorf("invalid qualifying stage")
		}
		q := a.currentQualifying(s.Model)
		for _, stage := range q.Stages {
			if stage.Kind == kind && stage.Model != nil {
				s.QualStage = kind
				s.QualStageSession = s.Model.Meta.ID
				return true, nil
			}
		}
		return true, fmt.Errorf("qualifying stage not recorded")
	}
	if handled, e := a.raceSelectionEvent(p.Event.Click.ID, p.Event.Submit.Form, p.Event.Submit.Values, p.Event.Submit.Changed); handled {
		return true, e
	}
	if p.Event.Click.ID == "analysis-edit-save" {
		s.EditSave = !s.EditSave
		return true, nil
	}
	if p.Event.Tab.ID == "analysis-tabs" {
		switch p.Event.Tab.Tab {
		case "traces", "laps", "tyres", "positions", "stats", "progression", "practice", "advanced":
			s.Tab = p.Event.Tab.Tab
		}
		return true, nil
	}
	if p.Event.Click.ID == "analysis-refresh" {
		for _, m := range a.rec.catalog() {
			if m.ID == s.Session {
				a.requestAnalysis(m, true)
				break
			}
		}
		return true, nil
	}
	vals := p.Event.Submit.Values
	if len(p.Event.Submit.Changed) > 0 {
		changed := map[string]json.RawMessage{}
		for _, key := range p.Event.Submit.Changed {
			if v, ok := vals[key]; ok {
				changed[key] = v
			}
		}
		vals = changed
	}
	if p.Event.Submit.Form == "analysis-filters" {
		keys := []string{"save", "season", "weekend", "session"}
		ptrs := []*string{&s.Save, &s.Season, &s.Weekend, &s.Session}
		for i, k := range keys {
			var v string
			if b, ok := vals[k]; ok && json.Unmarshal(b, &v) == nil && v != *ptrs[i] {
				s.EditSave = false
				s.Tab = "stats"
				*ptrs[i] = v
				for j := i + 1; j < len(ptrs); j++ {
					*ptrs[j] = ""
				}
				break
			}
		}
	}
	if p.Event.Submit.Form == "analysis-controls" {
		for k, ptr := range map[string]*int{"driver": &s.Driver, "lap": &s.Lap, "window": &s.Window} {
			var v string
			if b, ok := vals[k]; ok && json.Unmarshal(b, &v) == nil {
				n, e := strconv.Atoi(v)
				if e != nil || n < 0 || n > 255 {
					return true, fmt.Errorf("invalid analysis selection")
				}
				if (k == "driver" || k == "reference_driver") && n >= 24 {
					return true, fmt.Errorf("invalid driver")
				}
				if k == "window" && n > 4 {
					return true, fmt.Errorf("invalid range")
				}
				*ptr = n
			}
		}
		if b, ok := vals["reference_driver"]; ok {
			var v string
			if json.Unmarshal(b, &v) != nil {
				return true, fmt.Errorf("invalid comparison driver")
			}
			if v == "none" {
				s.Compare = false
				s.ReferenceLap = 0
			} else {
				n, e := strconv.Atoi(v)
				if e != nil || n < 0 || n >= 24 {
					return true, fmt.Errorf("invalid comparison driver")
				}
				if !s.Compare || s.ReferenceDriver != n {
					s.ReferenceLap = -1
				}
				s.ReferenceDriver = n
				s.Compare = true
			}
		}
		if b, ok := vals["reference_lap"]; ok && s.Compare {
			var v string
			if json.Unmarshal(b, &v) != nil {
				return true, fmt.Errorf("invalid comparison lap")
			}
			n, e := strconv.Atoi(v)
			if e != nil || n < 0 || n > 255 {
				return true, fmt.Errorf("invalid comparison lap")
			}
			s.ReferenceLap = n
			if n == 0 {
				s.Compare = false
			}
		}
	}
	if p.Event.Submit.Form == "analysis-save" {
		var name string
		if e := json.Unmarshal(vals["name"], &name); e != nil {
			return true, e
		}
		name = strings.TrimSpace(name)
		if e := a.rec.rename(s.Session, name); e != nil {
			return true, e
		}
		s.Save = name
		s.EditSave = false
	}
	return true, nil
}
