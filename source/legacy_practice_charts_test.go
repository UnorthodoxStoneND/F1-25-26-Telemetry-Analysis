package main

import (
	"fmt"
	"image"
	"image/draw"
	"math"
)

func renderTeamSectorLeaders(path string, m *AnalysisModel) error {
	leaders := teamSectorLeaders(m)
	f := newRaceFigure(3600, 620, "Team sector leaders", "Top five teams per sector · best recorded sector from valid completed laps · all drivers")
	for sector, teams := range leaders {
		left := 40 + sector*1190
		f.text(left, 115, 1100, 38, fmt.Sprintf("SECTOR %d", sector+1), 24, 600, raceMuted)
		if len(teams) == 0 {
			f.text(left, 190, 1100, 42, "No valid sector times recorded", 25, 400, raceMuted)
			continue
		}
		for row, v := range teams[:min(5, len(teams))] {
			y := 175 + row*78
			c := raceColours[v.Driver%len(raceColours)]
			f.text(left+10, y, 835, 40, v.Name, 25, 600, c)
			label := fmt.Sprintf("+%.3f", float64(v.Time-teams[0].Time)/1000)
			if row == 0 {
				label = fmt.Sprintf("%.3f s", float64(v.Time)/1000)
			}
			f.text(left+865, y, 270, 40, label, 26, 600, raceFG)
			chartLine(f.im, left+10, y+51, left+1130, y+51, raceGrid)
			length := int(1120 * float64(teams[0].Time) / float64(v.Time))
			chartLine(f.im, left+10, y+51, left+10+length, y+51, c)
		}
	}
	return f.save(path)
}

func renderPracticeDistribution(path string, m *AnalysisModel, selected []int) error {
	return renderLapDistributionWithCaption(path, m, selected, 3600, "Valid completed laps only · box: middle 50% / median · whiskers: 1.5×IQR")
}

func renderPracticeConsistency(path string, m *AnalysisModel, selected []int) error {
	f := newRaceFigure(3600, 900, "Consistency analysis", "Valid completed laps · box: mean ± standard deviation · whiskers: minimum / maximum · at least two laps required")
	f.category = true
	f.timeAxis = true
	f.plot.Max.Y -= 90
	low, high := math.Inf(1), math.Inf(-1)
	values := map[int][]float64{}
	for _, i := range selected {
		v := practiceValues(&m.Drivers[i])
		values[i] = v
		if len(v) >= 2 {
			low = math.Min(low, v[0])
			high = math.Max(high, v[len(v)-1])
		}
	}
	if !finite(low) {
		f.text(150, 230, 3200, 55, "At least two valid completed laps per driver are needed to measure consistency.", 29, 400, raceMuted)
		return f.save(path)
	}
	pad := math.Max(1, (high-low)*.1)
	f.axes(.5, float64(max(1, len(selected)))+.5, low-pad, high+pad, "Driver", "Lap time · M:SS", false)
	for n, i := range selected {
		x, _ := f.xy(float64(n+1), low)
		slot := f.plot.Dx() / max(1, len(selected))
		half := min(160, slot/3)
		c := raceColours[i%len(raceColours)]
		f.text(x-slot/2+10, f.plot.Max.Y+15, slot-20, 38, driverCode(m, i), 25, 600, c)
		v := values[i]
		if len(v) < 2 {
			f.text(x-slot/2+10, f.plot.Max.Y+55, slot-20, 35, "Insufficient laps", 21, 400, raceMuted)
			continue
		}
		mean, sd := practiceConsistency(v)
		_, yMin := f.xy(float64(n+1), v[0])
		_, yMax := f.xy(float64(n+1), v[len(v)-1])
		chartLine(f.im, x, yMin, x, yMax, c)
		chartLine(f.im, x-half/2, yMin, x+half/2, yMin, c)
		chartLine(f.im, x-half/2, yMax, x+half/2, yMax, c)
		_, y0 := f.xy(float64(n+1), math.Max(v[0], mean-sd))
		_, y1 := f.xy(float64(n+1), math.Min(v[len(v)-1], mean+sd))
		draw.Draw(f.im, image.Rect(x-half, y1, x+half, max(y0, y1+2)), &image.Uniform{c}, image.Point{}, draw.Src)
		_, mid := f.xy(float64(n+1), mean)
		chartLine(f.im, x-half, mid, x+half, mid, raceFG)
		f.text(x-slot/2+10, f.plot.Max.Y+55, slot-20, 35, fmt.Sprintf("SD %.3fs", sd), 21, 400, raceMuted)
		f.text(x-slot/2+10, f.plot.Max.Y+88, slot-20, 30, fmt.Sprintf("%d valid laps", len(v)), 18, 400, raceMuted)
	}
	return f.save(path)
}
