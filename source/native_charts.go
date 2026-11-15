package main

// These builders return the launcher's data-only Chart API. No renderer, image
// encoder, temporary chart file, or selection-keyed disk cache is used here.
import (
	"fmt"
	"math"
	"sort"
	"unicode/utf8"
)

func chartTextLimit(s string) string {
	for len(s) > 128 {
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
	}
	return s
}
func nativeColour(driver int) string {
	c := raceColours[driver%len(raceColours)]
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}
func nativeTyreColour(m *AnalysisModel, id int) string {
	c := tyreChartColour(m, id)
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}
func nativeAxis(label, format string) M { return M{"label": label, "format": format} }
func nativeCategory(labels []string, reverse bool) M {
	ticks := []M{}
	for i, s := range labels {
		ticks = append(ticks, M{"value": i, "label": chartTextLimit(s)})
	}
	return M{"domain": []float64{-.5, math.Max(.5, float64(len(labels))-.5)}, "ticks": ticks, "reversed": reverse}
}
func nativeChart(id, title, caption string, x, y M, height int) M {
	return M{"id": id, "title": title, "caption": caption, "height": min(1600, max(120, height)), "x": x, "y": y, "series": []M{}}
}
func nativeAdd(c M, s M) { c["series"] = append(c["series"].([]M), s) }
func nativeBlock(c M) M  { return M{"chart": c} }

type nativeXY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func nativePoint(x, y float64) nativeXY { return nativeXY{x, y} }
func nativeSeries(name, col, kind string) M {
	return M{"name": chartTextLimit(name), "color": col, "type": kind}
}
func nativeScatter(c M, name, col string, points []nativeXY, hollow bool) {
	if len(points) == 0 {
		return
	}
	s := nativeSeries(name, col, "scatter")
	s["points"], s["hollow"] = points, hollow
	nativeAdd(c, s)
}
func nativeLine(c M, name, col string, points []any, dashed, step, dots bool) {
	if len(points) == 0 {
		return
	}
	s := nativeSeries(name, col, "line")
	s["points"], s["dashed"], s["dots"] = points, dashed, dots
	s["interpolation"] = "linear"
	if step {
		s["interpolation"] = "step_after"
	}
	nativeAdd(c, s)
}
func nativeDriverLabels(m *AnalysisModel, ids []int) []string {
	out := []string{}
	for _, i := range ids {
		out = append(out, driverCode(m, i))
	}
	return out
}

// Preserve explicit gaps and pit-edge dash styles. Both styles share a legend.
func nativeRaceLine(c M, name, col string, points []racePoint, lapAxis, step, dots bool) {
	var layers [2][]any
	previousStyle := -1
	for n, p := range points {
		style := 0
		if p.Dashed {
			style = 1
		}
		if n == 0 || p.Break || (lapAxis && p.X-points[n-1].X > 1.01) {
			for j := range layers {
				if len(layers[j]) > 0 {
					layers[j] = append(layers[j], nil)
				}
			}
			layers[style] = append(layers[style], nativePoint(p.X, p.Y))
			previousStyle = style
			continue
		}
		prev := points[n-1]
		if prev.Dashed {
			style = 1
		}
		if previousStyle != style {
			layers[style] = append(layers[style], nil, nativePoint(prev.X, prev.Y))
		}
		layers[style] = append(layers[style], nativePoint(p.X, p.Y))
		previousStyle = style
	}
	for j, pts := range layers {
		if len(pts) > 0 {
			nativeLine(c, name, col, pts, j == 1, step, dots)
			name = ""
		}
	}
}

// Min/max buckets retain braking/speed peaks and run boundaries, with a bounded
// payload even for a 24-car, high-rate recording. Raw saved samples are untouched.
func nativeThinTrace(points []racePoint) []racePoint {
	if len(points) <= 2000 {
		return points
	}
	bucket := max(1, int(math.Ceil(float64(len(points))/450)))
	out := []racePoint{}
	for start := 0; start < len(points); {
		end := min(len(points), start+bucket)
		// Never combine samples across a genuine missing-data boundary.
		for n := start + 1; n < end; n++ {
			if points[n].Break {
				end = n
				break
			}
		}
		lo, hi := start, start
		for n := start + 1; n < end; n++ {
			if points[n].Y < points[lo].Y {
				lo = n
			}
			if points[n].Y > points[hi].Y {
				hi = n
			}
		}
		ids := []int{start, lo, hi, end - 1}
		sort.Ints(ids)
		last := -1
		for _, n := range ids {
			if n != last {
				out = append(out, points[n])
				last = n
			}
		}
		start = end
	}
	// Pathological entirely isolated data cannot be connected or allowed to grow
	// without bound. Evenly retain isolated points, always marking the gaps.
	if len(out) > 2000 {
		sampled := []racePoint{}
		stride := int(math.Ceil(float64(len(out)) / 2000))
		for n := 0; n < len(out); n += stride {
			p := out[n]
			p.Break = true
			sampled = append(sampled, p)
		}
		return sampled
	}
	return out
}

func nativeTraceChannel(id, title, unit string, m *AnalysisModel, selected []int, traces map[int][]TraceSample, ch int, domain *[2]float64) M {
	y := nativeAxis(unit, "number")
	if ch == 1 || ch == 2 {
		y["format"] = "percent"
	}
	if ch == 3 {
		y["format"] = "integer"
	}
	if domain != nil {
		y["domain"] = *domain
	}
	c := nativeChart(id, title, "", nativeAxis("Lap distance · m", "integer"), y, 190)
	for _, i := range selected {
		samples := traces[i]
		step := traceFrameStep(samples)
		points := make([]racePoint, 0, len(samples))
		for n, s := range samples {
			v := []float64{s.Speed, s.Throttle, s.Brake, s.Gear, s.RPM}
			p := racePoint{X: s.Distance, Y: v[ch]}
			if n > 0 {
				p.Break = !traceConnect(samples[n-1], s, step)
			}
			points = append(points, p)
		}
		nativeRaceLine(c, m.Drivers[i].name(i), nativeColour(i), nativeThinTrace(points), false, ch == 3, false)
	}
	return nativeBlock(c)
}

func nativeTyreTimeline(m *AnalysisModel, selected []int) M {
	c := nativeChart("tyre-strategy", "Tyre strategy", "Observed stints · partial ends show captured coverage only", nativeAxis("Race lap", "integer"), nativeCategory(nativeDriverLabels(m, selected), true), 180+len(selected)*30)
	c["x"].(M)["domain"] = []float64{0, float64(max(1, m.metrics().MaxLap))}
	byCompound := map[int][]M{}
	order := []int{}
	for row, i := range selected {
		for _, st := range m.raceStints(i) {
			if _, ok := byCompound[st.Compound]; !ok {
				order = append(order, st.Compound)
			}
			label := fmt.Sprintf("%s · %d laps", compound(st.Compound), st.End-st.Start+1)
			if st.Open {
				label += " · partial"
			}
			byCompound[st.Compound] = append(byCompound[st.Compound], M{"x": []float64{float64(st.Start-1) + .03, float64(st.End) - .03}, "y": []float64{float64(row) - .3, float64(row) + .3}, "label": label})
		}
	}
	for _, id := range order {
		s := nativeSeries(compound(id), nativeTyreColour(m, id), "ranges")
		s["cells"] = byCompound[id]
		nativeAdd(c, s)
	}
	return nativeBlock(c)
}
func nativePitAverages(m *AnalysisModel, selected []int) M {
	type mean struct {
		driver  int
		seconds float64
		count   int
	}
	means := []mean{}
	low, high := math.Inf(1), math.Inf(-1)
	for _, i := range selected {
		v, n := averagePitLane(&m.Drivers[i])
		means = append(means, mean{i, v, n})
		if n > 0 {
			low = math.Min(low, v)
			high = math.Max(high, v)
		}
	}
	sort.SliceStable(means, func(i, j int) bool {
		if means[i].count == 0 {
			return false
		}
		if means[j].count == 0 {
			return true
		}
		return means[i].seconds < means[j].seconds
	})
	labels := []string{}
	rows := [][]any{}
	for _, v := range means {
		labels = append(labels, driverCode(m, v.driver))
		time, gap := "—", "—"
		if v.count > 0 {
			time = fmt.Sprintf("%.3f s", v.seconds)
			gap = fmt.Sprintf("+%.3f s", v.seconds-low)
			if math.Abs(v.seconds-low) < .0005 {
				gap = "Best"
			}
		}
		rows = append(rows, []any{m.Drivers[v.driver].name(v.driver), time, gap, fmt.Sprint(v.count)})
	}
	c := nativeChart("pit-averages", "Average pit-stop duration", "Full pit-lane time · completed stationary stops · quickest first", nativeAxis("Seconds", "seconds"), nativeCategory(labels, true), max(280, len(means)*30))
	if finite(low) {
		pad := math.Max(.15, (high-low)*.15)
		start := math.Max(0, low-pad)
		c["x"].(M)["domain"] = []float64{start, high + pad}
		for row, v := range means {
			if v.count > 0 {
				nativeLine(c, m.Drivers[v.driver].name(v.driver), nativeColour(v.driver), []any{nativePoint(start, float64(row)), nativePoint(v.seconds, float64(row))}, false, false, false)
				nativeScatter(c, "", nativeColour(v.driver), []nativeXY{nativePoint(v.seconds, float64(row))}, false)
			}
		}
	}
	return raceCard("Pit lane averages", nativeBlock(c), tableBlock("native-pit-averages", []string{"Driver", "Mean pit lane", "Gap to best", "Stops"}, rows))
}
func nativeStintColumns(m *AnalysisModel, selected []int) M {
	c := nativeChart("stint-lengths", "Stint length by compound", "Segments follow each driver's chronological stint order", nativeCategory(nativeDriverLabels(m, selected), false), nativeAxis("Laps", "integer"), 320)
	by := map[int][]M{}
	order := []int{}
	for n, i := range selected {
		base := 0
		for _, st := range m.raceStints(i) {
			if _, ok := by[st.Compound]; !ok {
				order = append(order, st.Compound)
			}
			end := base + st.End - st.Start + 1
			by[st.Compound] = append(by[st.Compound], M{"position": n, "base": base, "value": end})
			base = end
		}
	}
	for _, id := range order {
		s := nativeSeries(compound(id), nativeTyreColour(m, id), "bars")
		s["width"], s["values"] = .6, by[id]
		nativeAdd(c, s)
	}
	return nativeBlock(c)
}
func nativeSectorDuel(m *AnalysisModel, selected []int) M {
	cells, totals := sectorDuelWinners(m, selected)
	c := nativeChart("sector-duel", "Sector duel · whole race", "Blank: a selected driver has no recorded sector. Ties share cells and win counts.", nativeAxis("Race lap", "integer"), nativeCategory([]string{"S1", "S2", "S3"}, true), 230)
	c["x"].(M)["domain"] = []float64{.5, float64(max(1, m.metrics().MaxLap)) + .5}
	by := map[int][]M{}
	stats := [][]any{}
	for lap := 1; lap <= m.metrics().MaxLap; lap++ {
		for sector, winners := range cells[lap] {
			for part, i := range winners {
				by[i] = append(by[i], M{"x": []float64{float64(lap) - .46, float64(lap) + .46}, "y": []float64{float64(sector) - .43 + .86*float64(part)/float64(len(winners)), float64(sector) - .43 + .86*float64(part+1)/float64(len(winners))}})
			}
		}
	}
	for _, i := range selected {
		if len(by[i]) > 0 {
			s := nativeSeries(m.Drivers[i].name(i), nativeColour(i), "heatmap")
			s["cells"] = by[i]
			nativeAdd(c, s)
		}
		stats = append(stats, []any{m.Drivers[i].name(i), fmt.Sprint(totals[0][i]), fmt.Sprint(totals[1][i]), fmt.Sprint(totals[2][i])})
	}
	return raceCard("Sector duel", nativeBlock(c), tableBlock("sector-wins", []string{"Driver", "S1 wins", "S2 wins", "S3 wins"}, stats))
}
func nativeDistribution(m *AnalysisModel, selected []int, practice, consistency bool) M {
	title, caption := "Lap-time distribution", "All completed timed laps · median / quartiles · 1.5×IQR whiskers · individual laps"
	if practice {
		caption = "Valid completed laps only · median / quartiles · 1.5×IQR whiskers · individual laps"
	}
	if consistency {
		title = "Consistency analysis"
		caption = "Valid completed laps · mean ± standard deviation · minimum / maximum whiskers · at least two laps"
	}
	c := nativeChart(map[bool]string{true: "consistency", false: "lap-distribution"}[consistency], title, caption, nativeCategory(nativeDriverLabels(m, selected), false), nativeAxis("Lap time", "lap_time"), 370)
	rows := [][]any{}
	for n, i := range selected {
		values := []float64{}
		if practice {
			values = practiceValues(&m.Drivers[i])
		} else {
			for _, l := range m.metrics().Laps[i] {
				if l.Time > 0 {
					values = append(values, float64(l.Time)/1000)
				}
			}
			sort.Float64s(values)
		}
		if len(values) == 0 || (consistency && len(values) < 2) {
			if consistency {
				rows = append(rows, []any{m.Drivers[i].name(i), "Insufficient valid laps", "—"})
			}
			continue
		}
		q1, median, q3 := quantile(values, .25), quantile(values, .5), quantile(values, .75)
		low, high := values[0], values[len(values)-1]
		if consistency {
			mean, sd := practiceConsistency(values)
			q1 = math.Max(low, mean-sd)
			q3 = math.Min(high, mean+sd)
			median = mean
			rows = append(rows, []any{m.Drivers[i].name(i), fmt.Sprintf("%.3f s", sd), fmt.Sprint(len(values))})
		} else {
			iqr := q3 - q1
			for _, v := range values {
				if v >= q1-1.5*iqr {
					low = v
					break
				}
			}
			for k := len(values) - 1; k >= 0; k-- {
				if values[k] <= q3+1.5*iqr {
					high = values[k]
					break
				}
			}
		}
		// Interpolated quartiles can sit beyond the outermost inlier in a
		// very small sample. Whiskers must still enclose the box itself.
		low, high = math.Min(low, q1), math.Max(high, q3)
		s := nativeSeries(m.Drivers[i].name(i), nativeColour(i), "boxes")
		s["width"] = .5
		s["values"] = []M{{"position": n, "min": low, "q1": q1, "median": median, "q3": q3, "max": high}}
		nativeAdd(c, s)
		if !consistency {
			dots := []nativeXY{}
			for k, v := range values {
				dots = append(dots, nativePoint(float64(n)+float64(k%7-3)*.025, v))
			}
			nativeScatter(c, "", nativeColour(i), dots, false)
		}
	}
	if consistency {
		return raceCard(title, nativeBlock(c), tableBlock("consistency-values", []string{"Driver", "Standard deviation", "Valid laps"}, rows))
	}
	return nativeBlock(c)
}
func nativeFieldPace(m *AnalysisModel, selected []int) M {
	field := cleanField(m)
	metrics := m.metrics()
	y := nativeAxis("Versus field median · negative is quicker", "signed_seconds")
	y["reversed"] = true
	c := nativeChart("field-pace", "Pace vs the field", "Clean-lap points · trailing three-lap trend · middle 50% of the whole field · excluded/missing laps break the lines", nativeAxis("Race lap", "integer"), y, 410)
	band := []any{}
	for lap := 1; lap <= metrics.MaxLap; lap++ {
		if q, ok := field[lap]; ok {
			band = append(band, M{"x": lap, "low": q.Low - q.Median, "high": q.High - q.Median})
		} else {
			band = append(band, nil)
		}
	}
	s := nativeSeries("Middle 50% of field", "#99AABA35", "band")
	s["points"] = band
	nativeAdd(c, s)
	nativeLine(c, "Field median", "#C6D0DD", []any{nativePoint(1, 0), nativePoint(float64(max(2, metrics.MaxLap)), 0)}, false, false, false)
	rows := [][]any{}
	for _, i := range selected {
		points := []racePoint{}
		dots := []nativeXY{}
		sum := 0.
		for lap := 1; lap <= metrics.MaxLap; lap++ {
			l := metrics.Laps[i][lap]
			q, ok := field[lap]
			if !ok || !cleanPaceLap(l) {
				continue
			}
			v := float64(l.Time)/1000 - q.Median
			points = append(points, racePoint{X: float64(lap), Y: v})
			dots = append(dots, nativePoint(float64(lap), v))
			sum += v
		}
		nativeScatter(c, "", nativeColour(i), dots, false)
		nativeRaceLine(c, m.Drivers[i].name(i), nativeColour(i), paceTrend(points), true, false, false)
		avg := "—"
		if len(points) > 0 {
			avg = fmt.Sprintf("%+.3f s", sum/float64(len(points)))
		}
		rows = append(rows, []any{m.Drivers[i].name(i), avg, fmt.Sprint(len(points))})
	}
	return raceCard("Pace vs the field", nativeBlock(c), tableBlock("field-pace-averages", []string{"Driver", "Average vs median", "Clean laps"}, rows))
}

func nativeTeamSectors(m *AnalysisModel) M {
	leaders := teamSectorLeaders(m)
	cards := []M{}
	for sector, teams := range leaders {
		teams = teams[:min(5, len(teams))]
		labels := []string{}
		for n := range teams {
			labels = append(labels, fmt.Sprintf("P%d", n+1))
		}
		c := nativeChart(fmt.Sprintf("team-sector-%d", sector), fmt.Sprintf("Sector %d leaders", sector+1), "Best captured sector from valid completed team laps", nativeAxis("Sector time · seconds", "seconds"), nativeCategory(labels, true), 260)
		rows := [][]any{}
		for n, v := range teams {
			delta := fmt.Sprintf("+%.3f s", float64(v.Time-teams[0].Time)/1000)
			if n == 0 {
				delta = "Fastest"
			}
			s := nativeSeries(v.Name, nativeColour(v.Driver), "bars")
			s["width"], s["horizontal"], s["values"] = .45, true, []M{{"position": n, "value": float64(v.Time) / 1000}}
			nativeAdd(c, s)
			rows = append(rows, []any{v.Name, sectorMS(v.Time), delta})
		}
		cards = append(cards, raceCard(fmt.Sprintf("Sector %d", sector+1), nativeBlock(c), tableBlock(fmt.Sprintf("team-sector-values-%d", sector), []string{"Team", "Time", "Gap"}, rows)))
	}
	return M{"grid": M{"columns": 3, "blocks": cards}}
}
func nativeQualifyingTimeline(q *QualifyingWeekend, stage int) M {
	s := q.Stages[stage]
	rows := qualifyingRows(s)
	visuals := q.visuals()
	labels := []string{}
	leader := uint32(0)
	for _, r := range rows {
		labels = append(labels, driverCode(s.Model, r.Driver))
		if r.Best > 0 && (leader == 0 || r.Best < leader) {
			leader = r.Best
		}
	}
	c := nativeChart("qualifying-timeline", "Qualifying timeline · "+sessionLabel(s.Kind), "Filled: advanced/final stage · hollow: confirmed elimination · grey: advancement unknown", nativeCategory(labels, false), nativeAxis("Gap to stage leader", "seconds"), 390)
	gapMax := .25
	values := [][]any{}
	for n, r := range rows {
		if r.Best == 0 {
			values = append(values, []any{s.Model.Drivers[r.Driver].name(r.Driver), "No time", "—"})
			continue
		}
		gap := float64(r.Best-leader) / 1000
		gapMax = math.Max(gapMax, gap)
		v := visuals[r.Key]
		col := nativeColour(v.Driver)
		state := q.advancement(stage, r.Key)
		status := "Advanced / final"
		if state < 0 {
			status = "Eliminated"
		}
		if state == 0 {
			col = "#99ABC3"
			status = "Unknown"
		}
		nativeLine(c, s.Model.Drivers[r.Driver].name(r.Driver), col, []any{nativePoint(float64(n), 0), nativePoint(float64(n), gap)}, false, false, false)
		nativeScatter(c, "", col, []nativeXY{nativePoint(float64(n), gap)}, state <= 0)
		values = append(values, []any{s.Model.Drivers[r.Driver].name(r.Driver), fmt.Sprintf("+%.3f s", gap), status})
	}
	if stage+1 < len(q.Stages) && s.Meta.State == "completed" && q.Stages[stage+1].Model != nil && q.Stages[stage+1].Meta.State == "completed" {
		cut := len(qualifyingRows(q.Stages[stage+1]))
		if cut > 0 && cut < len(rows) {
			nativeLine(c, fmt.Sprintf("Recorded cutoff · P%d", cut), "#F06274", []any{nativePoint(float64(cut)-.5, 0), nativePoint(float64(cut)-.5, gapMax*1.15)}, true, false, false)
		}
	}
	return raceCard("Qualifying timeline", nativeBlock(c), tableBlock("qualifying-timeline-values", []string{"Driver", "Gap", "Advancement"}, values))
}
func nativeKnockout(q *QualifyingWeekend) M {
	labels := []string{}
	ranks := make([]map[string]QualifyingRow, len(q.Stages))
	maxRank := 2
	visuals := q.visuals()
	order := []string{}
	seen := map[string]bool{}
	for n, s := range q.Stages {
		labels = append(labels, sessionLabel(s.Kind))
		ranks[n] = map[string]QualifyingRow{}
		for _, r := range qualifyingRows(s) {
			ranks[n][r.Key] = r
			maxRank = max(maxRank, r.Rank)
			if !seen[r.Key] {
				seen[r.Key] = true
				order = append(order, r.Key)
			}
		}
	}
	y := nativeAxis("Position", "integer")
	y["domain"], y["reversed"] = []int{1, maxRank}, true
	c := nativeChart("knockout-progression", "Knockout progression", "Teammates share a colour; second driver dashed · hollow: confirmed elimination · missing stages stay missing", nativeCategory(labels, false), y, max(390, maxRank*24))
	rows := [][]any{}
	for _, key := range order {
		v := visuals[key]
		points := []any{}
		eliminated := []nativeXY{}
		name := ""
		row := []any{}
		for n, s := range q.Stages {
			r, ok := ranks[n][key]
			if !ok {
				points = append(points, nil)
				row = append(row, "—")
				continue
			}
			if name == "" {
				name = s.Model.Drivers[r.Driver].name(r.Driver)
			}
			if n > 0 {
				if prev, ok := ranks[n-1][key]; ok {
					for step := 1; step < 16; step++ {
						t := float64(step) / 16
						ease := t * t * (3 - 2*t)
						points = append(points, nativePoint(float64(n-1)+t, float64(prev.Rank)+ease*float64(r.Rank-prev.Rank)))
					}
				}
			}
			points = append(points, nativePoint(float64(n), float64(r.Rank)))
			row = append(row, fmt.Sprint(r.Rank))
			if q.advancement(n, key) < 0 {
				eliminated = append(eliminated, nativePoint(float64(n), float64(r.Rank)))
			}
		}
		nativeLine(c, name, nativeColour(v.Driver), points, v.Dashed, false, false)
		nativeScatter(c, "", nativeColour(v.Driver), eliminated, true)
		rows = append(rows, append([]any{name}, row...))
	}
	return raceCard("Knockout progression", nativeBlock(c), tableBlock("knockout-ranks", append([]string{"Driver"}, labels...), rows))
}
func nativeTeammates(q *QualifyingWeekend) M {
	duels := q.teammateDuels()
	labels := []string{}
	for _, d := range duels {
		labels = append(labels, d.First+" > "+d.Second)
	}
	c := nativeChart("teammate-duels", "Teammate head-to-head", "Each driver's fastest valid lap across all recorded qualifying stages · winner first", nativeAxis("Teammate gap · seconds", "seconds"), nativeCategory(labels, true), max(280, len(duels)*30))
	rows := [][]any{}
	for n, d := range duels {
		s := nativeSeries(d.First+" > "+d.Second, nativeColour(d.Driver), "bars")
		s["width"], s["horizontal"], s["values"] = .5, true, []M{{"position": n, "value": float64(d.Gap) / 1000}}
		nativeAdd(c, s)
		rows = append(rows, []any{d.First, d.Second, fmt.Sprintf("+%.3f s", float64(d.Gap)/1000), d.Stage})
	}
	return raceCard("Teammate head-to-head", nativeBlock(c), tableBlock("teammate-gap-values", []string{"Leading driver", "Teammate", "Gap", "Stages"}, rows))
}

func nativeLegacyTraces(m *AnalysisModel, selection *AnalysisSelection, selected, comparison []TraceSample) []M {
	// Kept for legacy session kinds that expose the trace page; five separate
	// native graphics replace the former combined bitmap.
	out := []M{}
	titles := []string{"Speed", "Throttle", "Brake", "Gear", "Engine RPM"}
	units := []string{"km/h", "%", "%", "Gear", "RPM"}
	minD, maxD := 0., 0.
	for _, run := range [][]TraceSample{selected, comparison} {
		for _, s := range run {
			minD = math.Min(minD, s.Distance)
			maxD = math.Max(maxD, s.Distance)
		}
	}
	for ch := 0; ch < 5; ch++ {
		c := nativeChart(fmt.Sprintf("trace-channel-%d", ch), titles[ch], "", nativeAxis("Lap distance · m", "integer"), nativeAxis(units[ch], "number"), 190)
		if selection.Window >= 1 && selection.Window <= 4 && maxD > 0 {
			start, end := maxD*float64(selection.Window-1)/4, maxD*float64(selection.Window)/4
			if selection.Window == 1 {
				start = minD
			}
			c["x"].(M)["domain"] = []float64{start, end}
		}
		samples := [][]TraceSample{selected}
		names := []string{m.Drivers[selection.Driver].name(selection.Driver)}
		if selection.Compare {
			samples = append(samples, comparison)
			names = append(names, m.Drivers[selection.ReferenceDriver].name(selection.ReferenceDriver))
		}
		for j, run := range samples {
			pts := []racePoint{}
			step := traceFrameStep(run)
			for n, s := range run {
				v := []float64{s.Speed, s.Throttle, s.Brake, s.Gear, s.RPM}
				p := racePoint{X: s.Distance, Y: v[ch]}
				if n > 0 {
					p.Break = !traceConnect(run[n-1], s, step)
				}
				pts = append(pts, p)
			}
			col := []string{"#50BEFF", "#FFA752"}[j]
			nativeRaceLine(c, names[j], col, nativeThinTrace(pts), false, ch == 3, false)
		}
		out = append(out, nativeBlock(c))
	}
	return out
}

func nativeUnsupportedView() M {
	return M{"subtitle": "Telemetry Analysis · v" + addonVersion, "body": M{"blocks": []M{textBlock("Telemetry Analysis requires a launcher advertising native chart support (view.chart), available in Swofty 1.9.55. Update Swofty to view analysis. Live telemetry and session recording remain available here.")}}}
}

// Chart support is negotiated by capability rather than a launcher-version ceiling.
func nativeChartsSupported(capabilities []string) bool {
	for _, capability := range capabilities {
		if capability == "view.chart" {
			return true
		}
	}
	return false
}
