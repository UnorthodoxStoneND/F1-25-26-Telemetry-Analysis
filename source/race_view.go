package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (a *App) ensureRaceSelection(m *AnalysisModel) {
	s := &a.analysis
	if s.RaceSelectionSession == m.Meta.ID {
		return
	}
	s.RaceSelectionSession = m.Meta.ID
	s.RaceDrivers = 0
	s.RaceLap = 1
	ids := raceDriverIndices(m)
	for _, i := range ids {
		if i == m.Meta.Player {
			s.RaceDrivers = 1 << uint(i)
			return
		}
	}
	if len(ids) > 0 {
		s.RaceDrivers = 1 << uint(ids[0])
	}
}
func (a *App) selectedRaceDrivers(m *AnalysisModel) []int {
	out := []int{}
	for _, i := range raceDriverIndices(m) {
		if a.analysis.RaceDrivers&(1<<uint(i)) != 0 {
			out = append(out, i)
		}
	}
	return out
}
func (a *App) raceSelectionEvent(click, form string, values map[string]json.RawMessage, changed []string) (bool, error) {
	if !strings.HasPrefix(click, "analysis-race-") && !strings.HasPrefix(form, "analysis-race-") {
		return false, nil
	}
	m := a.analysis.Model
	if m == nil || (!raceSession(m.Meta.Session) && !practiceSession(m.Meta.Session)) {
		return true, nil
	}
	a.ensureRaceSelection(m)
	s := &a.analysis
	switch click {
	case "analysis-race-all":
		s.RaceDrivers = 0
		for _, i := range raceDriverIndices(m) {
			s.RaceDrivers |= 1 << uint(i)
		}
	case "analysis-race-none":
		s.RaceDrivers = 0
	}
	if suffix, ok := strings.CutPrefix(click, "analysis-race-driver-"+m.Meta.ID+"-"); ok {
		i, err := strconv.Atoi(suffix)
		if err == nil {
			for _, available := range raceDriverIndices(m) {
				if i == available {
					s.RaceDrivers ^= 1 << uint(i)
					break
				}
			}
		}
	}
	if form == "analysis-race-lap-"+m.Meta.ID {
		var text string
		if json.Unmarshal(values["lap"], &text) != nil {
			return true, fmt.Errorf("invalid race lap")
		}
		lap, e := strconv.Atoi(text)
		if e != nil || lap < 1 || lap > max(1, m.metrics().MaxLap) {
			return true, fmt.Errorf("invalid race lap")
		}
		s.RaceLap = lap
	}
	return true, nil
}
func (a *App) raceImage(m *AnalysisModel, kind string, render func(string) error) M {
	key := sha256.Sum256([]byte(fmt.Sprintf("%s/r2/%s/%d/%s/%x/%d/%s", addonVersion, m.Meta.ID, m.Frames, m.Meta.State, a.analysis.RaceDrivers, a.analysis.RaceLap, kind)))
	path := filepath.Join(a.data, "race-analysis-v3", fmt.Sprintf("%x.png", key[:16]))
	var cfgWidth, cfgHeight int
	read := func() error {
		file, e := os.Open(path)
		if e != nil {
			return e
		}
		defer file.Close()
		cfg, e := png.DecodeConfig(file)
		cfgWidth, cfgHeight = cfg.Width, cfg.Height
		return e
	}
	if e := read(); e != nil {
		if e = render(path); e != nil {
			return textBlock("Chart unavailable: " + e.Error())
		}
		if e = read(); e != nil {
			return textBlock("Chart unavailable: " + e.Error())
		}
	}
	display, e := a.displayImage(path)
	if e != nil {
		return textBlock("Chart unavailable: " + e.Error())
	}
	return M{"image": M{"path": display, "width": cfgWidth / 2, "height": cfgHeight / 2, "fit": "contain"}}
}
func raceCard(title string, blocks ...M) M { return M{"card": M{"title": title, "blocks": blocks}} }
func (a *App) raceLapSelector(m *AnalysisModel, label string) M {
	opts := []M{}
	last := max(1, m.metrics().MaxLap)
	for n := 1; n <= last; n++ {
		opts = append(opts, M{"value": fmt.Sprint(n), "label": fmt.Sprintf("Lap %d", n)})
	}
	if a.analysis.RaceLap < 1 || a.analysis.RaceLap > last {
		a.analysis.RaceLap = 1
	}
	return M{"form": M{"id": "analysis-race-lap-" + m.Meta.ID, "live": true, "columns": 1, "fields": []M{selectionField("lap", label, fmt.Sprint(a.analysis.RaceLap), opts)}}}
}
func (a *App) raceAnalysisBlocks(m *AnalysisModel) []M {
	a.ensureRaceSelection(m)
	selected := a.selectedRaceDrivers(m)
	s := &a.analysis
	out := a.analysisDriverChips(m)
	if len(selected) == 0 {
		return append(out, textBlock("No drivers selected. Choose any drivers or select the full grid."))
	}
	out = append(out, a.raceImage(m, "legend", func(path string) error { return renderRaceLegend(path, m, selected) }))
	switch s.Tab {
	case "traces":
		out = append(out, a.raceLapAnalysis(m, selected)...)
	case "tyres":
		out = append(out, a.raceImage(m, "strategy-timeline", func(path string) error { return renderTyreTimeline(path, m, selected) }))
		w, columns := 1800, 2
		if len(selected) > 10 {
			w, columns = 3600, 1
		}
		out = append(out, M{"grid": M{"columns": columns, "blocks": []M{a.raceImage(m, "pit-averages", func(path string) error { return renderPitAverages(path, m, selected, w) }), a.raceImage(m, "stint-columns", func(path string) error { return renderStintColumns(path, m, selected, w) })}}})
	case "positions":
		out = append(out, a.raceLapLines(m, selected, "position"), a.raceLapLines(m, selected, "gap"))

	case "progression":
		duel := a.raceImage(m, "sector-duel", func(path string) error { return renderSectorDuel(path, m, selected, 3600) })
		out = append(out, a.raceLapLines(m, selected, "lap-times"), duel, a.raceLapLines(m, selected, "pace"), a.raceImage(m, "distribution", func(path string) error { return renderLapDistribution(path, m, selected, 3600) }))
	}
	return out
}
func (a *App) raceLapAnalysis(m *AnalysisModel, selected []int) []M {
	out := []M{a.raceLapSelector(m, "Race lap")}
	lap := a.analysis.RaceLap
	missing := []string{}
	traces := map[int][]TraceSample{}
	for _, i := range selected {
		samples := m.Drivers[i].traceRun(lap, true)
		traces[i] = samples
		if len(samples) == 0 {
			missing = append(missing, m.Drivers[i].name(i))
		}
	}
	if len(missing) > 0 {
		out = append(out, textBlock("No recorded trace for this lap: "+strings.Join(missing, ", ")+"."))
	}
	charts := []M{}
	titles := []string{"Speed", "Throttle", "Brake", "Gear", "Engine RPM"}
	units := []string{"km/h", "%", "%", "Gear", "RPM"}
	limits := [][2]float64{{0, 360}, {0, 100}, {0, 100}, {0, 8}, {0, 15000}}
	for channel := 0; channel < 5; channel++ {
		ch := channel
		series := []raceSeries{}
		for _, i := range selected {
			data := raceSeries{Driver: i}
			samples := traces[i]
			step := traceFrameStep(samples)
			for n, s := range samples {
				values := []float64{s.Speed, s.Throttle, s.Brake, s.Gear, s.RPM}
				if ch == 0 && s.Speed > limits[ch][1] {
					limits[ch][1] = math.Ceil(s.Speed/40) * 40
				}
				if ch == 4 && s.RPM > limits[ch][1] {
					limits[ch][1] = math.Ceil(s.RPM/1000) * 1000
				}
				if ch == 3 && s.Gear < limits[ch][0] {
					limits[ch][0] = s.Gear
				}
				p := racePoint{X: s.Distance, Y: values[ch]}
				if n > 0 {
					p.Break = !traceConnect(samples[n-1], s, step)
				}
				data.Points = append(data.Points, p)
			}
			series = append(series, data)
		}
		kind := fmt.Sprintf("lap-%d-channel-%d", lap, ch)
		charts = append(charts, a.raceImage(m, kind, func(path string) error {
			return renderRaceLines(path, titles[ch], fmt.Sprintf("Race lap %d", lap), "Lap distance · m", units[ch], series, &limits[ch], false, false, ch == 3, 2200, 430)
		}))
	}
	rows := [][]any{}
	for _, i := range selected {
		r := m.metrics().Laps[i][lap]
		rows = append(rows, []any{m.Drivers[i].name(i), lapTime(r.Time), sectorMS(r.Sectors[0]), sectorMS(r.Sectors[1]), sectorMS(r.Sectors[2])})
	}
	times := tableBlock("race-lap-times", []string{"Driver", "Lap time", "S1", "S2", "S3"}, rows)
	columns := times["table"].(M)["columns"].([]M)
	for i, w := range []int{140, 94, 78, 78, 78} {
		columns[i]["width"] = w
	}
	out = append(out, M{"grid": M{"columns": []float64{3, 2}, "blocks": []M{raceCard("Telemetry", charts...), raceCard(fmt.Sprintf("Lap %d · lap and sector times", lap), times)}}})
	return out
}
func sectorMS(v uint32) string {
	if v == 0 {
		return "—"
	}
	return fmt.Sprintf("%.3f", float64(v)/1000)
}
func (a *App) raceLapLines(m *AnalysisModel, selected []int, kind string) M {
	metrics := m.metrics()
	all := []raceSeries{}
	gapMax := 1.
	for _, i := range selected {
		d := &m.Drivers[i]
		series := raceSeries{Driver: i}
		if kind == "position" && d.Grid > 0 {
			series.Points = append(series.Points, racePoint{X: 0, Y: float64(d.Grid)})
		}
		for lap := 1; lap <= metrics.MaxLap; lap++ {
			r := metrics.Laps[i][lap]
			if r.Time == 0 && ((kind != "position" && kind != "gap") || !d.Classified || lap > d.Completed) {
				continue
			}
			point := racePoint{X: float64(lap), Flags: r.Flags}
			switch kind {
			case "position":
				if r.Position <= 0 {
					continue
				}
				point.Y = float64(r.Position)
			case "gap":
				if r.Flags&flagGapKnown == 0 {
					continue
				}
				point.Y = float64(r.GapMS) / 1000
				gapMax = math.Max(gapMax, point.Y)
			case "lap-times":
				point.Y = float64(r.Time) / 1000
				point.Dashed = r.Flags&(flagPitIn|flagPitOut) != 0
			case "pace":
				median, ok := metrics.Median[lap]
				if !ok {
					continue
				}
				point.Y = float64(r.Time)/1000 - median
			}
			series.Points = append(series.Points, point)
		}
		all = append(all, series)
	}
	title, subtitle, unit := "Lap-time progression", "Completed timed laps · pit / SC / VSC / invalid laps remain included", "Lap time · seconds"
	var limits *[2]float64
	inverse, stepped := false, false
	switch kind {
	case "position":
		title, subtitle, unit = "Position flow", "Grid start, then latest captured position per lap · final classification supplies the finish", "Position"
		limit := [2]float64{1, float64(max(2, len(raceDriverIndices(m))))}
		limits = &limit
		inverse = true
		stepped = false
	case "gap":
		title, subtitle, unit = "Gap to race leader", "Last valid gap per lap · classified elapsed times supply missing finish gaps · other gaps stay blank", "Seconds behind leader"
		limit := [2]float64{0, gapMax * 1.1}
		limits = &limit
		inverse = true
	case "pace":
		title, subtitle, unit = "Pace vs the field", "Delta to the entire field's median on the same lap · all completed timed laps included", "Seconds vs field median"
	}
	return a.raceImage(m, kind, func(path string) error {
		if kind == "pace" {
			return renderFieldPace(path, m, selected)
		}
		if kind == "lap-times" {
			return renderLapProgression(path, m, selected, all)
		}
		w, h := 1800, 680
		if len(selected) > 10 || kind == "position" || kind == "gap" {
			w, h = 3600, 900
		}
		return renderRaceLines(path, title, subtitle, "Race lap", unit, all, limits, inverse, true, stepped, w, h)
	})
}
