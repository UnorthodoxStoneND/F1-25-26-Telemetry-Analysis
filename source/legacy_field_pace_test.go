package main

import (
	"fmt"
	"image/color"
	"math"
)

func raceDot(f *raceFigure, x, y, r int, c color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				f.im.SetRGBA(x+dx, y+dy, c)
			}
		}
	}
}
func roundedRaceBar(f *raceFigure, x0, y0, x1, y1 int, c color.RGBA) {
	r := min(12, (y1-y0)/2)
	if x1-x0 < 2*r {
		r = max(0, (x1-x0)/2)
	}
	for y := y0; y < y1; y++ {
		inset := 0
		if r > 0 {
			d := 0.
			if y < y0+r {
				d = float64(y0 + r - y)
			} else if y >= y1-r {
				d = float64(y - (y1 - r - 1))
			}
			if d > 0 {
				inset = r - int(math.Sqrt(math.Max(0, float64(r*r)-d*d)))
			}
		}
		if x1-inset > x0+inset {
			chartLine(f.im, x0+inset, y, x1-inset-1, y, c)
		}
	}
}
func renderFieldPace(path string, m *AnalysisModel, selected []int) error {
	field := cleanField(m)
	metrics := m.metrics()
	series := []raceSeries{}
	limit := .5
	for _, q := range field {
		limit = math.Max(limit, math.Max(math.Abs(q.Low-q.Median), math.Abs(q.High-q.Median)))
	}
	for _, driver := range selected {
		s := raceSeries{Driver: driver}
		for lap := 1; lap <= metrics.MaxLap; lap++ {
			l := metrics.Laps[driver][lap]
			q, ok := field[lap]
			if !ok || !cleanPaceLap(l) {
				continue
			}
			delta := float64(l.Time)/1000 - q.Median
			limit = math.Max(limit, math.Abs(delta))
			s.Points = append(s.Points, racePoint{X: float64(lap), Y: delta})
		}
		series = append(series, s)
	}
	tallyRows := (len(selected) + 5) / 6
	f := newRaceFigure(3600, 1040+tallyRows*40, "Pace vs the field", "Dots: clean racing laps · lines: trailing three-lap trend · shaded band: middle 50% of the whole field")
	f.inverse = true
	f.strongLines = true
	f.plot.Min.Y = 160 + tallyRows*40
	f.plot.Max.Y = f.im.Rect.Max.Y - 105
	f.axes(1, float64(max(2, metrics.MaxLap)), -limit*1.15, limit*1.15, "Race lap", "Seconds vs field median · negative is quicker", false)
	// Keep the axis description below the driver-average rows.
	f.texts[len(f.texts)-1].Y = f.plot.Min.Y - 42
	// Fill the field interquartile range without crossing unavailable laps.
	band := color.RGBA{33, 43, 56, 255}
	for lap := 1; lap < metrics.MaxLap; lap++ {
		a, okA := field[lap]
		b, okB := field[lap+1]
		if !okA || !okB {
			continue
		}
		x0, _ := f.xy(float64(lap), 0)
		x1, _ := f.xy(float64(lap+1), 0)
		for x := x0; x <= x1; x++ {
			t := float64(x-x0) / float64(max(1, x1-x0))
			low := (a.Low-a.Median)*(1-t) + (b.Low-b.Median)*t
			high := (a.High-a.Median)*(1-t) + (b.High-b.Median)*t
			_, y0 := f.xy(float64(lap), low)
			_, y1 := f.xy(float64(lap), high)
			chartLine(f.im, x, y0, x, y1, band)
		}
	}
	_, zero := f.xy(1, 0)
	chartLine(f.im, f.plot.Min.X, zero, f.plot.Max.X, zero, color.RGBA{198, 208, 221, 255})
	f.text(f.plot.Min.X+12, zero-35, 400, 32, "FIELD MEDIAN", 19, 600, raceFG)
	for n, s := range series {
		c := raceColours[s.Driver%len(raceColours)]
		sum := 0.
		for _, p := range s.Points {
			x, y := f.xy(p.X, p.Y)
			raceDot(f, x, y, 5, c)
			sum += p.Y
		}
		trend := raceSeries{Driver: s.Driver, Points: paceTrend(s.Points)}
		f.line(trend, true, false)
		label := driverCode(m, s.Driver) + "  —"
		if len(s.Points) > 0 {
			label = fmt.Sprintf("%s  %+.3f s", driverCode(m, s.Driver), sum/float64(len(s.Points)))
		}
		f.text(120+(n%6)*570, 112+(n/6)*40, 550, 36, label, 23, 600, c)
	}
	f.text(2400, 80, 1100, 30, "Average per clean lap · pit / SC / VSC / invalid laps excluded", 19, 400, raceMuted)
	return f.save(path)
}
