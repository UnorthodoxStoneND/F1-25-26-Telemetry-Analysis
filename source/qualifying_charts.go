package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
)

func renderQualifyingTimeline(path string, q *QualifyingWeekend, stage int) error {
	s := q.Stages[stage]
	rows := qualifyingRows(s)
	visuals := q.visuals()
	f := newRaceFigure(3600, 1000, "Qualifying timeline · "+sessionLabel(s.Kind), "Gap to the fastest valid lap · filled: advanced / final stage · hollow: confirmed elimination · grey ring: unknown")
	f.category = true
	f.axisPrecision = 3
	leader := uint32(0)
	maxGap := .25
	for _, r := range rows {
		if r.Best > 0 && (leader == 0 || r.Best < leader) {
			leader = r.Best
		}
	}
	if leader == 0 {
		f.text(150, 260, 3200, 60, "No valid qualifying times recorded for this stage", 30, 400, raceMuted)
		return f.save(path)
	}
	for _, r := range rows {
		if r.Best > 0 {
			maxGap = math.Max(maxGap, float64(r.Best-leader)/1000)
		}
	}
	f.axes(.5, float64(max(1, len(rows)))+.5, 0, maxGap*1.2, "Driver · fastest to slowest", "Gap to stage leader · seconds", false)
	slot := f.plot.Dx() / max(1, len(rows))
	for n, r := range rows {
		x, _ := f.xy(float64(n+1), 0)
		labelWidth := min(190, slot-8)
		v := visuals[r.Key]
		c := raceColours[v.Driver%len(raceColours)]
		f.text(x-labelWidth/2, f.plot.Max.Y+15, labelWidth, 35, driverCode(s.Model, r.Driver), 22, 600, c)
		if r.Best == 0 {
			f.text(x-labelWidth/2, f.plot.Max.Y-45, labelWidth, 35, "No time", 20, 400, raceMuted)
			continue
		}
		gap := float64(r.Best-leader) / 1000
		_, y := f.xy(float64(n+1), gap)
		chartLine(f.im, x, f.plot.Max.Y, x, y, c)
		state := q.advancement(stage, r.Key)
		raceDot(f, x, y, 13, c)
		if state <= 0 {
			raceDot(f, x, y, 9, raceBG)
		}
		if state == 0 {
			raceDot(f, x, y, 13, raceMuted)
			raceDot(f, x, y, 10, raceBG)
		}
		label := fmt.Sprintf("+%.3f", gap)
		if gap == 0 {
			label = "Fastest"
		}
		f.text(x-labelWidth/2, y-48, labelWidth, 36, label, 21, 600, raceFG)
	}
	if stage+1 < len(q.Stages) && s.Meta.State == "completed" && q.Stages[stage+1].Model != nil && q.Stages[stage+1].Meta.State == "completed" {
		cut := len(qualifyingRows(q.Stages[stage+1]))
		if cut > 0 && cut < len(rows) {
			x, _ := f.xy(float64(cut)+.5, 0)
			raceDashedLine(f.im, x, f.plot.Min.Y, x, f.plot.Max.Y, color.RGBA{240, 98, 116, 255})
			f.text(x+15, f.plot.Min.Y+12, 400, 38, fmt.Sprintf("Recorded cutoff · P%d", cut), 23, 600, raceFG)
		}
	}
	return f.save(path)
}
func qualCurve(f *raceFigure, x0, y0, x1, y1 int, c color.RGBA, dashed bool) {
	steps := max(1, (x1-x0)/2)
	px, py := x0, y0
	for n := 1; n <= steps; n++ {
		t := float64(n) / float64(steps)
		ease := t * t * (3 - 2*t)
		x, y := x0+int(t*float64(x1-x0)), y0+int(ease*float64(y1-y0))
		if !dashed || (n/7)%2 == 0 {
			chartLine(f.im, px, py, x, y, c)
			chartLine(f.im, px, py+1, x, y+1, c)
		}
		px, py = x, y
	}
}
func renderKnockoutProgression(path string, q *QualifyingWeekend) error {
	maxRows := 1
	for _, s := range q.Stages {
		maxRows = max(maxRows, len(qualifyingRows(s)))
	}
	h := max(460, 240+maxRows*38)
	f := newRaceFigure(3600, h, "Knockout progression", "Recorded stage positions · teammates share a color; second driver dashed · hollow: confirmed elimination · missing stages are not inferred")
	visuals := q.visuals()
	xAt := func(index int) int { return 210 + index*3060/max(1, len(q.Stages)-1) }
	yAt := func(rank int) int { return 190 + (rank-1)*38 }
	positions := make([]map[string]QualifyingRow, len(q.Stages))
	for index, s := range q.Stages {
		positions[index] = map[string]QualifyingRow{}
		x := xAt(index)
		f.text(x-40, 115, 270, 42, sessionLabel(s.Kind), 28, 600, raceFG)
		if s.Model == nil {
			f.text(x-50, 180, 300, 45, "Not recorded", 25, 400, raceMuted)
			continue
		}
		for _, r := range qualifyingRows(s) {
			positions[index][r.Key] = r
		}
	}
	for index := 0; index+1 < len(q.Stages); index++ {
		for key, r := range positions[index] {
			if next, ok := positions[index+1][key]; ok {
				v := visuals[key]
				qualCurve(f, xAt(index), yAt(r.Rank), xAt(index+1), yAt(next.Rank), raceColours[v.Driver%len(raceColours)], v.Dashed)
			}
		}
	}
	for index, s := range q.Stages {
		for _, r := range qualifyingRows(s) {
			x, y := xAt(index), yAt(r.Rank)
			v := visuals[r.Key]
			c := raceColours[v.Driver%len(raceColours)]
			state := q.advancement(index, r.Key)
			if state == -1 {
				draw.Draw(f.im, image.Rect(x-19, y-18, x+19, y+18), &image.Uniform{color.RGBA{69, 31, 43, 255}}, image.Point{}, draw.Src)
			}
			raceDot(f, x, y, 8, c)
			if state == -1 {
				raceDot(f, x, y, 5, raceBG)
			}
			if index == 0 {
				f.text(15, y-17, 185, 36, driverCode(s.Model, r.Driver), 22, 600, c)
			}
			continues := false
			if index+1 < len(positions) {
				_, continues = positions[index+1][r.Key]
			}
			if !continues {
				label := fmt.Sprintf("%s  P%d", driverCode(s.Model, r.Driver), r.Rank)
				if state == 0 && index+1 < len(q.Stages) {
					label += " · pending"
				}
				f.text(x+25, y-17, 305, 36, label, 22, 400, raceFG)
			}
		}
	}
	return f.save(path)
}
func renderTeammateDuels(path string, q *QualifyingWeekend) error {
	duels := q.teammateDuels()
	h := max(640, 225+len(duels)*74)
	f := newRaceFigure(3600, h, "Teammate head-to-head", "Each driver's fastest valid lap across all recorded qualifying stages · winner first · stages may differ")
	if len(duels) == 0 {
		f.text(150, 250, 3200, 60, "No teammate pair has two valid recorded lap times.", 29, 400, raceMuted)
		return f.save(path)
	}
	maxGap := .1
	for _, d := range duels {
		maxGap = math.Max(maxGap, float64(d.Gap)/1000)
	}
	left, right, top, bottom := 480, 3020, 165, h-105
	for tick := 0; tick <= 5; tick++ {
		x := left + tick*(right-left)/5
		chartLine(f.im, x, top-20, x, bottom, raceGrid)
		f.text(x-30, bottom+15, 130, 35, fmt.Sprintf("%.2f", maxGap*1.15*float64(tick)/5), 22, 400, raceMuted)
	}
	for row, d := range duels {
		y := top + int((float64(row)+.5)*float64(bottom-top)/float64(len(duels)))
		c := raceColours[d.Driver%len(raceColours)]
		f.text(20, y-19, 440, 40, d.First+" > "+d.Second, 23, 600, c)
		end := left + int(float64(d.Gap)/1000/(maxGap*1.15)*float64(right-left))
		draw.Draw(f.im, image.Rect(left, y-18, max(left+3, end), y+18), &image.Uniform{c}, image.Point{}, draw.Src)
		f.text(end+18, y-19, 440, 40, fmt.Sprintf("+%.3f s · %s", float64(d.Gap)/1000, d.Stage), 23, 400, raceFG)
	}
	f.text(left, h-42, right-left, 35, "Teammate gap · seconds", 23, 400, raceMuted)
	return f.save(path)
}
