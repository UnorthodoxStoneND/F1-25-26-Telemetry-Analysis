package main

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type circuitInfo struct{ Country, Circuit string }

var circuits = map[int]circuitInfo{
	0: {"Australia", "Albert Park"}, 2: {"China", "Shanghai"}, 3: {"Bahrain", "Bahrain International"}, 4: {"Spain", "Barcelona-Catalunya"}, 5: {"Monaco", "Monaco"}, 6: {"Canada", "Gilles Villeneuve"}, 7: {"United Kingdom", "Silverstone"}, 9: {"Hungary", "Hungaroring"}, 10: {"Belgium", "Spa-Francorchamps"}, 11: {"Italy", "Monza"}, 12: {"Singapore", "Marina Bay"}, 13: {"Japan", "Suzuka"}, 14: {"United Arab Emirates", "Yas Marina"}, 15: {"United States", "Circuit of the Americas"}, 16: {"Brazil", "Interlagos"}, 17: {"Austria", "Red Bull Ring"}, 19: {"Mexico", "Hermanos Rodríguez"}, 20: {"Azerbaijan", "Baku City Circuit"}, 26: {"Netherlands", "Zandvoort"}, 27: {"Italy", "Imola"}, 29: {"Saudi Arabia", "Jeddah Corniche"}, 30: {"United States", "Miami International"}, 31: {"United States", "Las Vegas Strip"}, 32: {"Qatar", "Lusail"}, 39: {"United Kingdom", "Silverstone (Reverse)"}, 40: {"Austria", "Red Bull Ring (Reverse)"}, 41: {"Netherlands", "Zandvoort (Reverse)"}, 42: {"Spain", "MADRING"},
}

func resultTabLabel(session int) string {
	if session >= 5 && session <= 14 {
		return "Qualifying Results"
	}
	return "Overview"
}
func sessionSummary(m *AnalysisModel) []M {
	track := m.Meta.Track
	air, surface, weather, laps := "—", "—", "—", "—"
	if c := m.Conditions; c != nil {
		track = c.Track
		air = fmt.Sprintf("%d°C", c.Air)
		surface = fmt.Sprintf("%d°C", c.Surface)
		names := []string{"Clear", "Light cloud", "Overcast", "Light rain", "Heavy rain", "Storm"}
		if c.Weather >= 0 && c.Weather < len(names) {
			weather = names[c.Weather]
		}
		if c.TotalLaps > 0 && m.Meta.Session >= 15 && m.Meta.Session <= 17 {
			laps = fmt.Sprint(c.TotalLaps)
		}
	}
	if m.Meta.Session >= 1 && m.Meta.Session <= 14 {
		laps = "Timed session"
	}
	info, known := circuits[track]
	if !known {
		info = circuitInfo{"—", fmt.Sprintf("Track %d", track)}
		if track < 0 {
			info.Circuit = "—"
		}
	}
	return []M{fact("Country", info.Country), fact("Circuit", info.Circuit), fact("Laps", laps), fact("Air temp", air), fact("Track temp", surface), fact("Weather", weather)}
}
func (a *App) summaryBlock(m *AnalysisModel) M {
	data := a.data
	values := sessionSummary(m)
	if data == "" {
		return M{"stats": values}
	}
	key := sha256.Sum256([]byte(fmt.Sprint(values)))
	dir := filepath.Join(data, "session-summary-v1")
	path := filepath.Join(dir, fmt.Sprintf("%x.png", key[:10]))
	if _, e := os.Stat(path); e != nil {
		if e = os.MkdirAll(dir, 0755); e != nil {
			return M{"stats": values}
		}
		if e = renderSessionSummary(path, values); e != nil {
			return M{"stats": values}
		}
	}
	display, e := a.displayImage(path)
	if e != nil {
		return M{"stats": values}
	}
	return M{"image": M{"path": display, "width": 1800, "height": 72, "fit": "contain"}}
}

var summaryAccents = []color.RGBA{{72, 200, 187, 255}, {170, 148, 245, 255}, {93, 182, 239, 255}, {235, 191, 92, 255}, {235, 145, 95, 255}, {109, 193, 185, 255}}

type summaryText struct {
	Text                     string
	X, Y, W, H, Size, Weight int
	Colour                   color.RGBA
}

func roundedPanel(im *image.RGBA, r image.Rectangle, radius int, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cx, cy := x, y
			if x < r.Min.X+radius {
				cx = r.Min.X + radius
			}
			if x >= r.Max.X-radius {
				cx = r.Max.X - radius - 1
			}
			if y < r.Min.Y+radius {
				cy = r.Min.Y + radius
			}
			if y >= r.Max.Y-radius {
				cy = r.Max.Y - radius - 1
			}
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= radius*radius {
				im.SetRGBA(x, y, c)
			}
		}
	}
}
func summaryIcon(im *image.RGBA, index, x, y int, c color.RGBA) {
	line := func(a, b, d, e int) { chartLine(im, x+a*2, y+b*2, x+d*2, y+e*2, c) }
	circle := func(cx, cy, r int) {
		for angle := 0.; angle < 2*math.Pi; angle += .05 {
			a, b := cx+int(math.Cos(angle)*float64(r)), cy+int(math.Sin(angle)*float64(r))
			d, e := cx+int(math.Cos(angle+.05)*float64(r)), cy+int(math.Sin(angle+.05)*float64(r))
			line(a, b, d, e)
		}
	}
	switch index {
	case 0:
		circle(0, 0, 11)
		line(-11, 0, 11, 0)
		line(-8, -6, 8, -6)
		line(-8, 6, 8, 6)
		for _, sx := range []int{-1, 1} {
			for t := -10; t < 10; t++ {
				a := int(5 * math.Sqrt(math.Max(0, 1-float64(t*t)/121)))
				b := int(5 * math.Sqrt(math.Max(0, 1-float64((t+1)*(t+1))/121)))
				line(sx*a, t, sx*b, t+1)
			}
		}
	case 1:
		points := [][2]int{{-11, 7}, {-6, -8}, {2, -11}, {10, -4}, {7, 6}, {-1, 9}, {-5, 2}, {-11, 7}}
		for i := 1; i < len(points); i++ {
			line(points[i-1][0], points[i-1][1], points[i][0], points[i][1])
		}
		circle(7, 6, 2)
	case 2:
		line(-9, -12, -9, 12)
		line(-9, -10, 10, -10)
		line(10, -10, 7, 1)
		line(7, 1, -9, 1)
		line(-3, -10, -3, 1)
		line(3, -10, 3, 1)
		line(-9, -5, 8, -5)
	case 3:
		circle(-4, -2, 6)
		for _, p := range [][2]int{{-4, -13}, {-4, 9}, {-15, -2}, {7, -2}, {-12, -10}, {4, -10}} {
			line(p[0], p[1], p[0]+1, p[1]+1)
		}
		line(7, 2, 7, 11)
		line(11, 2, 11, 11)
		circle(9, 11, 3)
	case 4:
		line(-11, 12, -6, -12)
		line(11, 12, 6, -12)
		line(0, -10, 0, -5)
		line(0, -1, 0, 4)
		line(0, 8, 0, 12)
	case 5, 7, 8:
		circle(-5, -2, 5)
		circle(2, -5, 7)
		circle(9, 0, 4)
		line(-10, 3, 12, 3)
		if index == 5 {
			line(-6, 8, -8, 12)
			line(1, 8, -1, 12)
			line(8, 8, 6, 12)
		}
		if index == 8 {
			line(3, 6, -2, 11)
			line(-2, 11, 3, 11)
			line(3, 11, -1, 15)
		}
	case 6:
		circle(0, 0, 7)
		for a := 0.; a < 2*math.Pi; a += math.Pi / 4 {
			line(int(10*math.Cos(a)), int(10*math.Sin(a)), int(13*math.Cos(a)), int(13*math.Sin(a)))
		}
	}
}
func renderSessionSummary(path string, values []M) error {
	im := image.NewRGBA(image.Rect(0, 0, 3600, 144))
	draw.Draw(im, im.Bounds(), &image.Uniform{color.RGBA{22, 27, 35, 255}}, image.Point{}, draw.Src)
	texts := []summaryText{}
	for i, v := range values {
		left := i * 600
		accent := summaryAccents[i]
		roundedPanel(im, image.Rect(left+4, 6, left+592, 138), 16, color.RGBA{47, 57, 73, 255})
		roundedPanel(im, image.Rect(left+6, 8, left+590, 136), 14, color.RGBA{26, 33, 45, 255})
		chartLine(im, left+28, 10, left+568, 10, accent)
		roundedPanel(im, image.Rect(left+24, 40, left+92, 108), 16, color.RGBA{34, 45, 59, 255})
		icon := i
		if i == 5 {
			switch v["value"] {
			case "Clear":
				icon = 6
			case "Light cloud", "Overcast", "—":
				icon = 7
			case "Storm":
				icon = 8
			}
		}
		summaryIcon(im, icon, left+58, 74, accent)
		texts = append(texts, summaryText{Text: v["label"].(string), X: left + 112, Y: 29, W: 456, H: 30, Size: 21, Weight: 600, Colour: accent}, summaryText{Text: v["value"].(string), X: left + 112, Y: 66, W: 456, H: 46, Size: 36, Weight: 600, Colour: color.RGBA{235, 241, 249, 255}})
	}
	if e := drawSummaryText(im, texts); e != nil {
		return e
	}
	f, e := os.Create(path + ".tmp")
	if e != nil {
		return e
	}
	e = png.Encode(f, im)
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	return os.Rename(path+".tmp", path)
}
