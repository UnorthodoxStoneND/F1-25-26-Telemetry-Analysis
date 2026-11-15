package main

import "fmt"

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

func (a *App) summaryBlock(m *AnalysisModel) M { return M{"stats": sessionSummary(m)} }
