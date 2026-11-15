package main

import (
	"sort"
)

type fieldLap struct{ Median, Low, High float64 }

func cleanPaceLap(l RaceLapMetric) bool {
	return l.Time > 0 && l.Flags&(flagPitIn|flagPitOut|flagSC|flagVSC|flagInvalid) == 0
}
func cleanField(m *AnalysisModel) map[int]fieldLap {
	values := map[int][]float64{}
	for _, laps := range m.metrics().Laps {
		for n, l := range laps {
			if cleanPaceLap(l) {
				values[n] = append(values[n], float64(l.Time)/1000)
			}
		}
	}
	out := map[int]fieldLap{}
	for n, v := range values {
		sort.Float64s(v)
		out[n] = fieldLap{quantile(v, .5), quantile(v, .25), quantile(v, .75)}
	}
	return out
}
func paceTrend(points []racePoint) []racePoint {
	out := make([]racePoint, 0, len(points))
	for i, p := range points {
		sum, count := p.Y, 1
		for j := i - 1; j >= 0 && count < 3 && points[j+1].X-points[j].X == 1; j-- {
			sum += points[j].Y
			count++
		}
		p.Y = sum / float64(count)
		out = append(out, p)
	}
	return out
}
