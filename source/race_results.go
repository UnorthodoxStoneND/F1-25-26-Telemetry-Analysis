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
	"strings"
)

func clockDuration(seconds float64) string {
	if seconds <= 0 || !finite(seconds) {
		return "—"
	}
	n := int(math.Round(seconds))
	return fmt.Sprintf("%02d:%02d:%02d", n/3600, n/60%60, n%60)
}

// Only expand recognized constructor-grid identities; retain custom names.
func resultDriverName(d RecordedDriver, index int) string {
	names := map[string]struct {
		name   string
		number int
	}{
		"NORRIS": {"Lando Norris", 4}, "PIASTRI": {"Oscar Piastri", 81},
		"VERSTAPPEN": {"Max Verstappen", 3}, "HADJAR": {"Isack Hadjar", 6},
		"LECLERC": {"Charles Leclerc", 16}, "HAMILTON": {"Lewis Hamilton", 44},
		"RUSSELL": {"George Russell", 63}, "ANTONELLI": {"Kimi Antonelli", 12},
		"ALBON": {"Alexander Albon", 23}, "SAINZ": {"Carlos Sainz", 55},
		"LAWSON": {"Liam Lawson", 30}, "LINDBLAD": {"Arvid Lindblad", 41},
		"ALONSO": {"Fernando Alonso", 14}, "STROLL": {"Lance Stroll", 18},
		"GASLY": {"Pierre Gasly", 10}, "COLAPINTO": {"Franco Colapinto", 43},
		"BEARMAN": {"Oliver Bearman", 87}, "OCON": {"Esteban Ocon", 31},
		"HULKENBERG": {"Nico Hulkenberg", 27}, "HÜLKENBERG": {"Nico Hülkenberg", 27},
		"BORTOLETO": {"Gabriel Bortoleto", 5}, "BOTTAS": {"Valtteri Bottas", 77},
		"PÉREZ": {"Sergio Pérez", 11}, "PEREZ": {"Sergio Pérez", 11},
		"BROWNING": {"Luke Browning", 7}, "GOETHE": {"Oliver Goethe", 83},
		"TSUNODA": {"Yuki Tsunoda", 22}, "DOOHAN": {"Jack Doohan", 7},
		"RICCIARDO": {"Daniel Ricciardo", 3}, "ZHOU": {"Zhou Guanyu", 24}, "SARGEANT": {"Logan Sargeant", 2},
	}
	name := d.name(index)
	if strings.EqualFold(strings.TrimSpace(name), "VERSTAPPEN") && (d.RaceNumber == 1 || d.RaceNumber == 33) {
		return "Max Verstappen"
	}
	if v, ok := names[strings.ToUpper(strings.TrimSpace(name))]; ok && d.RaceNumber == v.number {
		return v.name
	}
	return name
}
func (m *AnalysisModel) resultGap(index int) string {
	d := m.Drivers[index]
	if d.Classified {
		switch d.ResultStatus {
		case 4, 7:
			return "DNF"
		case 5:
			return "DSQ"
		case 6:
			return "Not classified"
		}
		for _, winner := range m.Drivers {
			if !winner.Classified || winner.Position != 1 || winner.RaceSeconds <= 0 {
				continue
			}
			if d.Position == 1 {
				return "Winner"
			}
			if d.Completed < winner.Completed {
				return fmt.Sprintf("+%d lap%s", winner.Completed-d.Completed, map[bool]string{true: "", false: "s"}[winner.Completed-d.Completed == 1])
			}
			if d.RaceSeconds > 0 && d.Completed == winner.Completed {
				gap := d.RaceSeconds + float64(d.PenaltySeconds) - winner.RaceSeconds - float64(winner.PenaltySeconds)
				if gap >= 0 {
					return fmt.Sprintf("+%.3f s", gap)
				}
			}
		}
	}
	return "—"
}
func (m *AnalysisModel) usedCompounds(index int) string {
	labels := []string{}
	seen := map[int]bool{}
	for _, st := range m.raceStints(index) {
		if !seen[st.Compound] {
			seen[st.Compound] = true
			labels = append(labels, compound(st.Compound))
		}
	}
	if len(labels) == 0 {
		return "—"
	}
	return strings.Join(labels, " · ")
}
func (a *App) podiumPosition(pos int) any {
	if pos < 1 || pos > 3 || a.displayCache == "" {
		return knownInt(pos)
	}
	path := filepath.Join(a.data, "results-v4", fmt.Sprintf("position-%d.png", pos))
	if _, e := os.Stat(path); e != nil {
		im := image.NewRGBA(image.Rect(0, 0, 60, 52))
		draw.Draw(im, im.Bounds(), &image.Uniform{raceBG}, image.Point{}, draw.Src)
		c := []color.RGBA{{232, 190, 79, 255}, {195, 205, 218, 255}, {195, 133, 83, 255}}[pos-1]
		if drawSummaryText(im, []summaryText{{Text: fmt.Sprint(pos), X: 5, Y: 1, W: 50, H: 50, Size: 36, Weight: 700, Colour: c}}) != nil {
			return knownInt(pos)
		}
		if os.MkdirAll(filepath.Dir(path), 0755) != nil {
			return knownInt(pos)
		}
		file, e := os.Create(path)
		if e != nil {
			return knownInt(pos)
		}
		e = png.Encode(file, im)
		_ = file.Close()
		if e != nil {
			return knownInt(pos)
		}
	}
	display, e := a.displayImage(path)
	if e != nil {
		return knownInt(pos)
	}
	return M{"text": "", "sort": pos, "image": M{"path": display, "width": 30, "height": 26}}
}
