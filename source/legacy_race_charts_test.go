package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type raceFigure struct {
	im             *image.RGBA
	texts          []summaryText
	plot           image.Rectangle
	x0, x1, y0, y1 float64
	inverse        bool
	category       bool
	timeAxis       bool
	strongLines    bool
	axisPrecision  int
}

func newRaceFigure(w, h int, title, subtitle string) *raceFigure {
	f := &raceFigure{im: image.NewRGBA(image.Rect(0, 0, w, h)), plot: image.Rect(120, 140, w-62, h-88)}
	draw.Draw(f.im, f.im.Bounds(), &image.Uniform{raceBG}, image.Point{}, draw.Src)
	f.text(38, 18, w-76, 44, title, 32, 600, raceFG)
	if subtitle != "" {
		f.text(38, 64, w-76, 32, subtitle, 21, 400, raceMuted)
	}
	return f
}
func (f *raceFigure) text(x, y, w, h int, s string, size, weight int, c color.RGBA) {
	f.texts = append(f.texts, summaryText{Text: s, X: x, Y: y, W: w, H: h, Size: size, Weight: weight, Colour: c})
}
func (f *raceFigure) xy(x, y float64) (int, int) {
	xx := f.plot.Min.X + int((x-f.x0)/(f.x1-f.x0)*float64(f.plot.Dx()))
	yy := f.plot.Max.Y - int((y-f.y0)/(f.y1-f.y0)*float64(f.plot.Dy()))
	if f.inverse {
		yy = f.plot.Min.Y + int((y-f.y0)/(f.y1-f.y0)*float64(f.plot.Dy()))
	}
	return xx, yy
}
func (f *raceFigure) axes(x0, x1, y0, y1 float64, xlabel, ylabel string, integers bool) {
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	f.x0, f.x1, f.y0, f.y1 = x0, x1, y0, y1
	yTicks := 5
	if integers && (y1-y0 <= 10 || ylabel == "Position") {
		yTicks = max(1, int(math.Ceil(y1-y0)))
	}
	for tick := 0; tick <= yTicks; tick++ {
		v := y0 + (y1-y0)*float64(tick)/float64(yTicks)
		if integers {
			v = math.Round(v)
		}
		_, y := f.xy(x0, v)
		chartLine(f.im, f.plot.Min.X, y, f.plot.Max.X, y, raceGrid)
		label := fmt.Sprintf("%.1f", v)
		if f.axisPrecision > 0 {
			label = fmt.Sprintf("%.*f", f.axisPrecision, v)
		}
		if f.timeAxis {
			n := int(math.Round(v))
			label = fmt.Sprintf("%d:%02d", n/60, n%60)
		}
		if integers {
			label = fmt.Sprintf("%.0f", v)
		}
		f.text(6, y-18, 104, 36, label, 23, 400, raceMuted)
	}
	xValues := []float64{}
	if xlabel == "Race lap" {
		step := math.Max(1, math.Ceil((x1-x0)/8))
		for v := math.Ceil(x0); v <= x1; v += step {
			xValues = append(xValues, v)
		}
		if len(xValues) > 0 && xValues[len(xValues)-1] != x1 {
			xValues = append(xValues, x1)
		}
	} else {
		for tick := 0; tick <= 8; tick++ {
			xValues = append(xValues, x0+(x1-x0)*float64(tick)/8)
		}
	}
	for _, v := range xValues {
		if f.category {
			break
		}
		x, _ := f.xy(v, y0)
		chartLine(f.im, x, f.plot.Min.Y, x, f.plot.Max.Y, raceGrid)
		f.text(x-30, f.plot.Max.Y+12, 100, 34, fmt.Sprintf("%.0f", v), 22, 400, raceMuted)
	}
	f.text(f.plot.Min.X, f.im.Rect.Max.Y-38, f.plot.Dx(), 30, xlabel, 22, 400, raceMuted)
	f.text(f.plot.Min.X, 106, 500, 30, ylabel, 21, 400, raceMuted)
}
func (f *raceFigure) line(series raceSeries, lapAxis bool, stepped bool) {
	c := raceColours[series.Driver%len(raceColours)]
	var previous racePoint
	last := false
	for _, p := range series.Points {
		if !finite(p.X) || !finite(p.Y) || p.X < f.x0 || p.X > f.x1 || p.Y < f.y0 || p.Y > f.y1 {
			last = false
			continue
		}
		x, y := f.xy(p.X, p.Y)
		if last && !p.Break && (!lapAxis || p.X-previous.X <= 1) {
			px, py := f.xy(previous.X, previous.Y)
			if stepped {
				chartLine(f.im, px, py, x, py, c)
				chartLine(f.im, x, py, x, y, c)
			} else if p.Dashed || previous.Dashed {
				raceDashedLine(f.im, px, py, x, y, c)
			} else {
				chartLine(f.im, px, py, x, y, c)
				if f.strongLines {
					chartLine(f.im, px, py-1, x, y-1, c)
					chartLine(f.im, px, py+1, x, y+1, c)
				}
			}
		}
		draw.Draw(f.im, image.Rect(x-2, y-2, x+3, y+3), &image.Uniform{c}, image.Point{}, draw.Src)
		previous = p
		last = true
	}
}
func (f *raceFigure) save(path string) error {
	if e := drawSummaryText(f.im, f.texts); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	file, e := os.CreateTemp(filepath.Dir(path), ".race-chart-*")
	if e != nil {
		return e
	}
	temp := file.Name()
	defer os.Remove(temp)
	e = png.Encode(file, f.im)
	if c := file.Close(); e == nil {
		e = c
	}
	if e != nil {
		return e
	}
	return os.Rename(temp, path)
}
func renderRaceLines(path, title, subtitle, xlabel, ylabel string, series []raceSeries, fixedY *[2]float64, inverse, lapAxis, stepped bool, width, height int) error {
	return raceLineFigure(title, subtitle, xlabel, ylabel, series, fixedY, inverse, lapAxis, stepped, width, height).save(path)
}
func raceLineFigure(title, subtitle, xlabel, ylabel string, series []raceSeries, fixedY *[2]float64, inverse, lapAxis, stepped bool, width, height int) *raceFigure {
	f := newRaceFigure(width, height, title, subtitle)
	f.strongLines = ylabel == "Position"
	f.inverse = inverse
	x0, x1, y0, y1 := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, s := range series {
		for _, p := range s.Points {
			if !finite(p.X) || !finite(p.Y) {
				continue
			}
			x0 = math.Min(x0, p.X)
			x1 = math.Max(x1, p.X)
			y0 = math.Min(y0, p.Y)
			y1 = math.Max(y1, p.Y)
		}
	}
	if !finite(x0) {
		f.text(140, 210, width-220, 70, "No recorded measurements for this selection", 28, 400, raceMuted)
		return f
	}
	if lapAxis {
		x0 = math.Min(1, x0)
		x1 = math.Ceil(x1)
	}
	if fixedY != nil {
		y0, y1 = fixedY[0], fixedY[1]
	} else {
		padding := math.Max((y1-y0)*.08, .25)
		y0 -= padding
		y1 += padding
	}
	f.axes(x0, x1, y0, y1, xlabel, ylabel, stepped || ylabel == "Position")
	for _, s := range series {
		f.line(s, lapAxis, stepped)
	}
	return f
}
func raceDashedLine(im *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx, dy := float64(x1-x0), float64(y1-y0)
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	for start := 0.; start < length; start += 20 {
		end := math.Min(start+11, length)
		chartLine(im, x0+int(math.Round(dx*start/length)), y0+int(math.Round(dy*start/length)), x0+int(math.Round(dx*end/length)), y0+int(math.Round(dy*end/length)), c)
	}
}
func renderLapProgression(path string, m *AnalysisModel, selected []int, series []raceSeries) error {
	const width, chartHeight = 3600, 1000
	f := raceLineFigure("Lap-by-lap progression", "Dashed: pit in/out laps · best: valid laps · average: all completed timed laps", "Race lap", "Lap time · seconds", series, nil, false, true, false, width, chartHeight)
	rows := (len(selected) + 3) / 4
	im := image.NewRGBA(image.Rect(0, 0, width, chartHeight+rows*140+20))
	draw.Draw(im, im.Bounds(), &image.Uniform{raceBG}, image.Point{}, draw.Src)
	draw.Draw(im, f.im.Bounds(), f.im, image.Point{}, draw.Src)
	f.im = im
	for n, driver := range selected {
		x, y := 30+(n%4)*885, chartHeight+(n/4)*140
		draw.Draw(im, image.Rect(x, y, x+855, y+116), &image.Uniform{color.RGBA{24, 32, 44, 255}}, image.Point{}, draw.Src)
		c := raceColours[driver%len(raceColours)]
		draw.Draw(im, image.Rect(x+16, y+18, x+21, y+98), &image.Uniform{c}, image.Point{}, draw.Src)
		best, average := raceLapSummary(m, driver)
		f.text(x+40, y+12, 790, 38, m.Drivers[driver].name(driver), 27, 600, c)
		f.text(x+40, y+61, 385, 38, "BEST  "+lapTime(best), 24, 600, raceFG)
		f.text(x+435, y+61, 395, 38, "AVG  "+lapTime(average), 24, 400, raceMuted)
	}
	return f.save(path)
}
func renderRaceLegend(path string, m *AnalysisModel, selected []int) error {
	rows := (len(selected) + 3) / 4
	h := rows*52 + 24
	f := newRaceFigure(3600, h, "", "")
	f.texts = nil
	for k, i := range selected {
		x := 24 + (k%4)*900
		y := 14 + (k/4)*52
		c := raceColours[i%24]
		chartLine(f.im, x, y+16, x+52, y+16, c)
		f.text(x+68, y, 810, 42, driverCode(m, i)+"  "+m.Drivers[i].name(i), 26, 600, raceFG)
	}
	return f.save(path)
}
func renderTyreTimeline(path string, m *AnalysisModel, selected []int) error {
	h := 220 + len(selected)*52
	f := newRaceFigure(3600, h, "Tyre strategy", "Observed compounds and stint boundaries · open ends show captured coverage only")
	left, right := 420, 3530
	maxLap := m.metrics().MaxLap
	if maxLap < 1 {
		maxLap = 1
	}
	for tick := 0; tick <= 10; tick++ {
		lap := float64(maxLap) * float64(tick) / 10
		x := left + int(lap/float64(maxLap)*float64(right-left))
		chartLine(f.im, x, 150, x, h-80, raceGrid)
		f.text(x-15, 108, 100, 38, fmt.Sprintf("%.0f", lap), 24, 400, raceMuted)
	}
	for row, i := range selected {
		y := 158 + row*52
		f.text(24, y+4, 380, 44, driverCode(m, i)+"  "+m.Drivers[i].name(i), 25, 600, raceFG)
		stints := m.raceStints(i)
		if len(stints) == 0 {
			f.text(left, y+4, 1000, 44, "No recorded stint data", 24, 400, raceMuted)
		}
		for _, st := range stints {
			x0 := left + int(float64(st.Start-1)/float64(maxLap)*float64(right-left))
			x1 := left + int(float64(st.End)/float64(maxLap)*float64(right-left))
			c := tyreChartColour(m, st.Compound)
			roundedRaceBar(f, x0+3, y+4, x1-3, y+38, c)
			label := fmt.Sprintf("%s · %d laps", compound(st.Compound), st.End-st.Start+1)
			if st.Open {
				label += " · partial"
			}
			if x1-x0 > 135 {
				f.text(x0+16, y+4, x1-x0-25, 32, label, 21, 600, color.RGBA{20, 27, 38, 255})
			}
		}
	}
	f.compounds(m, selected, h-50)
	return f.save(path)
}
func (f *raceFigure) compounds(m *AnalysisModel, selected []int, y int) {
	seen := map[int]bool{}
	ids := []int{}
	for _, i := range selected {
		for _, st := range m.raceStints(i) {
			if !seen[st.Compound] {
				seen[st.Compound] = true
				ids = append(ids, st.Compound)
			}
		}
	}
	f.text(24, y, 170, 36, "Compounds", 22, 600, raceMuted)
	spacing := min(220, (f.im.Rect.Max.X-230)/max(1, len(ids)))
	for k, id := range ids {
		x := 210 + k*spacing
		draw.Draw(f.im, image.Rect(x, y+3, x+30, y+29), &image.Uniform{tyreChartColour(m, id)}, image.Point{}, draw.Src)
		f.text(x+43, y-1, spacing-48, 38, compound(id), 24, 600, raceFG)
	}
}
func renderPitAverages(path string, m *AnalysisModel, selected []int, w int) error {
	type pitMean struct {
		driver  int
		seconds float64
		count   int
	}
	means := []pitMean{}
	low, high := math.Inf(1), math.Inf(-1)
	for _, driver := range selected {
		v, n := averagePitLane(&m.Drivers[driver])
		means = append(means, pitMean{driver, v, n})
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
	h := max(640, 250+len(means)*80)
	f := newRaceFigure(w, h, "Average pit-stop duration", "Full pit-lane time · completed stops · quickest first")
	left, right, top, bottom := 230, w-420, 160, h-105
	if !finite(low) {
		f.text(left, top, w-left-40, 50, "No completed timed pit stops for the selected drivers", 25, 400, raceMuted)
		return f.save(path)
	}
	pad := math.Max(.15, (high-low)*.15)
	start, end := math.Max(0, low-pad), high+pad
	for tick := 0; tick <= 4; tick++ {
		v := start + (end-start)*float64(tick)/4
		x := left + int(float64(right-left)*float64(tick)/4)
		chartLine(f.im, x, top-15, x, bottom, raceGrid)
		f.text(x-40, bottom+15, 110, 35, fmt.Sprintf("%.2f", v), 22, 400, raceMuted)
	}
	for row, v := range means {
		y := top + int((float64(row)+.5)*float64(bottom-top)/float64(max(1, len(means))))
		c := raceColours[v.driver%len(raceColours)]
		f.text(24, y-18, 195, 38, driverCode(m, v.driver), 24, 600, c)
		if v.count == 0 {
			f.text(right+35, y-18, 360, 38, "— no timed stop", 22, 400, raceMuted)
			continue
		}
		x := left + int((v.seconds-start)/(end-start)*float64(right-left))
		chartLine(f.im, left, y, x, y, c)
		raceDot(f, x, y, 11, c)
		label := fmt.Sprintf("%.2f s  +%.2f", v.seconds, v.seconds-low)
		if math.Abs(v.seconds-low) < .0005 {
			label = fmt.Sprintf("%.2f s  best", v.seconds)
		}
		f.text(right+35, y-18, 365, 38, label, 23, 600, raceFG)
	}
	f.text(left, h-42, right-left, 35, "Seconds · mean total pit-lane duration", 22, 400, raceMuted)
	return f.save(path)
}

func renderStintColumns(path string, m *AnalysisModel, selected []int, w int) error {
	f := newRaceFigure(w, 640, "Stint length by compound", "One column per driver · segments follow stint order · labels show compound and laps")
	f.category = true
	f.plot.Max.Y -= 60
	maxLaps := 1
	for _, i := range selected {
		sum := 0
		for _, st := range m.raceStints(i) {
			sum += st.End - st.Start + 1
		}
		maxLaps = max(maxLaps, sum)
	}
	f.axes(.5, float64(max(1, len(selected)))+.5, 0, float64(maxLaps), "Driver key (see legend)", "Laps", true)
	// Replace generic numeric x ticks with the selected driver keys.
	for k, i := range selected {
		x, _ := f.xy(float64(k+1), 0)
		slot := f.plot.Dx() / max(1, len(selected))
		half := min(82, slot/3)
		bottom := 0
		for _, st := range m.raceStints(i) {
			length := st.End - st.Start + 1
			_, y0 := f.xy(float64(k+1), float64(bottom))
			_, y1 := f.xy(float64(k+1), float64(bottom+length))
			c := tyreChartColour(m, st.Compound)
			draw.Draw(f.im, image.Rect(x-half, y1+2, x+half, y0-2), &image.Uniform{c}, image.Point{}, draw.Src)
			if y0-y1 > 45 {
				f.text(x-half+8, y1+10, half*2-12, 36, fmt.Sprintf("%s · %d", compound(st.Compound), length), 23, 600, color.RGBA{20, 27, 38, 255})
			}
			bottom += length
		}
		draw.Draw(f.im, image.Rect(x-slot/2, f.plot.Max.Y+2, x+slot/2, f.plot.Max.Y+48), &image.Uniform{raceBG}, image.Point{}, draw.Src)
		f.text(x-slot/2+3, f.plot.Max.Y+10, slot-6, 38, driverCode(m, i), 23, 600, raceFG)
	}
	f.compounds(m, selected, f.im.Rect.Max.Y-90)
	return f.save(path)
}

// Compare the same sector on the same race lap. Missing selected-driver data
// leaves that cell unknown; ties share both the cell and the win count.
func renderSectorDuel(path string, m *AnalysisModel, selected []int, w int) error {
	cells, totals := sectorDuelWinners(m, selected)
	tallyRows := (len(selected) + 3) / 4
	top := 170 + tallyRows*34
	h := top + 340
	f := newRaceFigure(w, h, "Sector duel · whole race", "Fastest selected driver in each sector on each lap · totals count sector wins, including shared ties")
	left, right := 120, w-50
	colWidth := (right - left) / 3
	for sector := 0; sector < 3; sector++ {
		drivers := append([]int(nil), selected...)
		sort.SliceStable(drivers, func(i, j int) bool { return totals[sector][drivers[i]] > totals[sector][drivers[j]] })
		f.text(left+sector*colWidth, 112, colWidth-15, 35, fmt.Sprintf("Sector %d wins", sector+1), 23, 600, raceFG)
		for n, driver := range drivers {
			x := left + sector*colWidth + (n%4)*((colWidth-15)/4)
			y := 150 + (n/4)*34
			f.text(x, y, (colWidth-15)/4-4, 32, fmt.Sprintf("%s: %d", driverCode(m, driver), totals[sector][driver]), 18, 600, raceColours[driver%len(raceColours)])
		}
	}
	laps := max(1, m.metrics().MaxLap)
	for sector := 0; sector < 3; sector++ {
		y := top + sector*72
		f.text(24, y+18, 88, 35, fmt.Sprintf("S%d", sector+1), 25, 600, raceMuted)
		for lap := 1; lap <= laps; lap++ {
			x0 := left + (lap-1)*(right-left)/laps
			x1 := left + lap*(right-left)/laps
			draw.Draw(f.im, image.Rect(x0+1, y, x1-1, y+66), &image.Uniform{raceGrid}, image.Point{}, draw.Src)
			winners := cells[lap][sector]
			for n, driver := range winners {
				y0 := y + n*66/len(winners)
				y1 := y + (n+1)*66/len(winners)
				draw.Draw(f.im, image.Rect(x0+1, y0, x1-1, y1), &image.Uniform{raceColours[driver%len(raceColours)]}, image.Point{}, draw.Src)
			}
		}
	}
	tickStep := max(1, int(math.Ceil(float64(laps)/12)))
	for lap := 1; lap <= laps; lap++ {
		if lap != 1 && lap != laps && lap%tickStep != 0 {
			continue
		}
		x := left + int((float64(lap)-.5)*float64(right-left)/float64(laps))
		f.text(x-25, top+222, 70, 32, fmt.Sprint(lap), 21, 400, raceMuted)
	}
	f.text(left, top+262, 300, 32, "Race lap", 22, 400, raceMuted)
	f.text(left, h-43, right-left, 35, "Blank: a selected driver has no recorded sector time · tied winners share a cell · pit / SC / VSC / invalid laps remain included", 18, 400, raceMuted)
	return f.save(path)
}

func renderLapDistribution(path string, m *AnalysisModel, selected []int, w int) error {
	return renderLapDistributionWithCaption(path, m, selected, w, "Box: median / middle 50% · whiskers: 1.5×IQR · every completed timed lap is included")
}
func renderLapDistributionWithCaption(path string, m *AnalysisModel, selected []int, w int, caption string) error {
	f := newRaceFigure(w, 800, "Lap-time distribution", caption)
	f.category = true
	f.timeAxis = true
	y0, y1 := math.Inf(1), math.Inf(-1)
	metrics := m.metrics()
	values := map[int][]float64{}
	for _, i := range selected {
		for _, l := range metrics.Laps[i] {
			if l.Time > 0 {
				v := float64(l.Time) / 1000
				values[i] = append(values[i], v)
				y0 = math.Min(y0, v)
				y1 = math.Max(y1, v)
			}
		}
		sort.Float64s(values[i])
	}
	if !finite(y0) {
		f.text(150, 220, w-250, 60, "No completed timed laps for this selection", 28, 400, raceMuted)
		return f.save(path)
	}
	pad := math.Max(1, (y1-y0)*.1)
	f.axes(.5, float64(max(1, len(selected)))+.5, y0-pad, y1+pad, "Driver key (see legend)", "Lap time · M:SS", false)
	for k, i := range selected {
		x, _ := f.xy(float64(k+1), y0)
		slot := f.plot.Dx() / max(1, len(selected))
		half := min(70, slot/4)
		v := values[i]
		if len(v) == 0 {
			continue
		}
		q1, median, q3 := quantile(v, .25), quantile(v, .5), quantile(v, .75)
		iqr := q3 - q1
		low, high := v[0], v[len(v)-1]
		for _, p := range v {
			if p >= q1-1.5*iqr {
				low = p
				break
			}
		}
		for j := len(v) - 1; j >= 0; j-- {
			if v[j] <= q3+1.5*iqr {
				high = v[j]
				break
			}
		}
		_, a := f.xy(float64(k+1), q1)
		_, b := f.xy(float64(k+1), q3)
		_, lo := f.xy(float64(k+1), low)
		_, hi := f.xy(float64(k+1), high)
		_, med := f.xy(float64(k+1), median)
		c := raceColours[i%24]
		draw.Draw(f.im, image.Rect(x-half, b, x+half, max(a, b+3)), &image.Uniform{color.RGBA{c.R / 3, c.G / 3, c.B / 3, 255}}, image.Point{}, draw.Src)
		chartLine(f.im, x, hi, x, lo, c)
		chartLine(f.im, x-half, hi, x+half, hi, c)
		chartLine(f.im, x-half, lo, x+half, lo, c)
		chartLine(f.im, x-half, med, x+half, med, c)
		for n, l := range metrics.Laps[i] {
			if l.Time == 0 {
				continue
			}
			_, y := f.xy(float64(k+1), float64(l.Time)/1000)
			jitter := (n%7 - 3) * 4
			draw.Draw(f.im, image.Rect(x+jitter-2, y-2, x+jitter+3, y+3), &image.Uniform{c}, image.Point{}, draw.Src)
		}
		draw.Draw(f.im, image.Rect(x-slot/2, f.plot.Max.Y+2, x+slot/2, f.plot.Max.Y+48), &image.Uniform{raceBG}, image.Point{}, draw.Src)
		f.text(x-slot/2+3, f.plot.Max.Y+10, slot-6, 38, driverCode(m, i), 23, 600, raceFG)
	}
	return f.save(path)
}
