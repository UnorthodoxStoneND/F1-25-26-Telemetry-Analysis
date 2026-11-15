package main

import (
	"image/color"
	"math"
)

var raceColours = []color.RGBA{
	{81, 190, 255, 255}, {255, 168, 83, 255}, {184, 153, 255, 255}, {72, 215, 166, 255}, {255, 105, 137, 255}, {239, 214, 94, 255},
	{97, 225, 235, 255}, {229, 151, 217, 255}, {149, 198, 104, 255}, {255, 129, 97, 255}, {129, 159, 255, 255}, {197, 218, 237, 255},
	{210, 175, 100, 255}, {111, 217, 188, 255}, {200, 134, 255, 255}, {239, 184, 154, 255}, {127, 201, 239, 255}, {225, 130, 179, 255},
	{175, 208, 150, 255}, {250, 199, 116, 255}, {149, 159, 206, 255}, {92, 195, 200, 255}, {198, 183, 224, 255}, {218, 219, 147, 255},
}
var raceBG = color.RGBA{17, 23, 33, 255}
var raceFG = color.RGBA{222, 230, 242, 255}
var raceMuted = color.RGBA{153, 171, 195, 255}
var raceGrid = color.RGBA{46, 60, 79, 255}

type racePoint struct {
	X, Y   float64
	Flags  uint8
	Break  bool
	Dashed bool
}
type raceSeries struct {
	Driver int
	Points []racePoint
}

func raceLapSummary(m *AnalysisModel, driver int) (uint32, uint32) {
	var best uint32
	var total uint64
	count := uint64(0)
	for _, lap := range m.metrics().Laps[driver] {
		if lap.Time == 0 {
			continue
		}
		total += uint64(lap.Time)
		count++
		if lap.Flags&flagInvalid == 0 && (best == 0 || lap.Time < best) {
			best = lap.Time
		}
	}
	if count == 0 {
		return 0, 0
	}
	return best, uint32(math.Round(float64(total) / float64(count)))
}

func tyreChartColour(m *AnalysisModel, compoundID int) color.RGBA {
	visual := 0
	if compoundID >= 0 && compoundID < len(m.CompoundVisual) {
		visual = m.CompoundVisual[compoundID]
	}
	switch visual {
	case 16:
		return color.RGBA{241, 87, 109, 255}
	case 17:
		return color.RGBA{239, 207, 83, 255}
	case 18:
		return color.RGBA{222, 231, 243, 255}
	case 7:
		return color.RGBA{75, 206, 140, 255}
	case 8:
		return color.RGBA{76, 157, 236, 255}
	}
	if compoundID == 7 {
		return color.RGBA{75, 206, 140, 255}
	}
	if compoundID == 8 {
		return color.RGBA{76, 157, 236, 255}
	}
	return color.RGBA{142, 156, 180, 255}
}

func sectorDuelWinners(m *AnalysisModel, selected []int) (map[int][3][]int, [3]map[int]int) {
	cells := map[int][3][]int{}
	totals := [3]map[int]int{{}, {}, {}}
	metrics := m.metrics()
	for lap := 1; lap <= metrics.MaxLap; lap++ {
		var sectors [3][]int
		for sector := 0; sector < 3; sector++ {
			best := uint32(math.MaxUint32)
			complete := len(selected) > 0
			for _, driver := range selected {
				value := metrics.Laps[driver][lap].Sectors[sector]
				if value == 0 {
					complete = false
					break
				}
				if value < best {
					best = value
					sectors[sector] = []int{driver}
				} else if value == best {
					sectors[sector] = append(sectors[sector], driver)
				}
			}
			if !complete {
				sectors[sector] = nil
			}
			for _, driver := range sectors[sector] {
				totals[sector][driver]++
			}
		}
		cells[lap] = sectors
	}
	return cells, totals
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := float64(len(sorted)-1) * q
	lo := int(index)
	hi := min(lo+1, len(sorted)-1)
	return sorted[lo] + (sorted[hi]-sorted[lo])*(index-float64(lo))
}
