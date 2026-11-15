package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strings"
)

var chartGlyphs = map[rune][]string{
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G': {"01111", "10000", "10000", "10111", "10001", "10001", "01110"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'-': {"00000", "00000", "00000", "11111", "00000", "00000", "00000"},
	'%': {"11001", "11010", "00100", "00100", "01000", "10110", "00110"},
	'/': {"00001", "00010", "00010", "00100", "01000", "01000", "10000"},
}

func chartText(im *image.RGBA, x, y int, s string, c color.RGBA) {
	for _, ch := range strings.ToUpper(s) {
		g := glyphs[ch]
		if g == nil {
			g = chartGlyphs[ch]
		}
		for yy, row := range g {
			for xx, b := range row {
				if b == '1' {
					draw.Draw(im, image.Rect(x+xx*2, y+yy*2, x+xx*2+2, y+yy*2+2), &image.Uniform{c}, image.Point{}, draw.Src)
				}
			}
		}
		x += 12
	}
}
func chartLine(im *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx, dy := int(math.Abs(float64(x1-x0))), -int(math.Abs(float64(y1-y0)))
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	e := dx + dy
	for {
		if image.Pt(x0, y0).In(im.Bounds()) {
			im.SetRGBA(x0, y0, c)
			im.SetRGBA(x0, y0+1, c)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}
func renderTrace(path string, a, b []TraceSample, window int) error {
	const width, height = 1600, 1120
	im := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(im, im.Bounds(), &image.Uniform{color.RGBA{17, 22, 31, 255}}, image.Point{}, draw.Src)
	fg := color.RGBA{177, 190, 211, 255}
	grid := color.RGBA{44, 55, 72, 255}
	blue := color.RGBA{80, 190, 255, 255}
	orange := color.RGBA{255, 167, 82, 255}
	maxD, minD := 0.0, 0.0
	for _, samples := range [][]TraceSample{a, b} {
		for _, s := range samples {
			maxD = math.Max(maxD, s.Distance)
			minD = math.Min(minD, s.Distance)
		}
	}
	if maxD <= 0 {
		return fmt.Errorf("no distance samples")
	}
	start, end := minD, maxD
	if window >= 1 && window <= 4 {
		start = maxD * float64(window-1) / 4
		end = maxD * float64(window) / 4
		if window == 1 {
			start = minD
		}
	}
	labels := []string{"SPEED KM/H", "THROTTLE %", "BRAKE %", "GEAR", "RPM"}
	limits := []float64{360, 100, 100, 8, 15000}
	const left, right, top, rowH = 120, 1550, 60, 205
	for channel := range labels {
		y := top + channel*rowH
		chartText(im, left, y-28, labels[channel], fg)
		for tick := 0; tick <= 4; tick++ {
			yy := y + 150 - tick*150/4
			chartLine(im, left, yy, right, yy, grid)
			chartText(im, 12, yy-6, fmt.Sprintf("%d", int(limits[channel]*float64(tick)/4)), fg)
		}
		for tick := 0; tick <= 8; tick++ {
			x := left + (right-left)*tick/8
			chartLine(im, x, y, x, y+150, grid)
			if channel == 4 {
				chartText(im, x-26, y+166, fmt.Sprintf("%d", int(start+(end-start)*float64(tick)/8)), fg)
			}
		}
		for trace, samples := range [][]TraceSample{a, b} {
			col := blue
			if trace == 1 {
				col = orange
			}
			lastX, lastY := -1, -1
			frameStep := traceFrameStep(samples)
			var previous TraceSample
			for _, s := range samples {
				if s.Distance < start || s.Distance > end {
					lastX = -1
					continue
				}
				values := []float64{s.Speed, s.Throttle, s.Brake, s.Gear, s.RPM}
				v := math.Max(0, math.Min(limits[channel], values[channel]))
				x := left + int((s.Distance-start)/(end-start)*float64(right-left))
				yy := y + 150 - int(v/limits[channel]*150)
				im.SetRGBA(x, yy, col)
				if lastX >= 0 && traceConnect(previous, s, frameStep) {
					if channel == 3 {
						chartLine(im, lastX, lastY, x, lastY, col)
						chartLine(im, x, lastY, x, yy, col)
					} else {
						chartLine(im, lastX, lastY, x, yy, col)
					}
				}
				lastX, lastY, previous = x, yy, s
			}
		}
	}
	chartText(im, 650, 1090, "LAP DISTANCE M", fg)
	f, e := os.Create(path + ".tmp")
	if e != nil {
		return e
	}
	e = png.Encode(f, im)
	c := f.Close()
	if e == nil {
		e = c
	}
	if e != nil {
		return e
	}
	return os.Rename(path+".tmp", path)
}
