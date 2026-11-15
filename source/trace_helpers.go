package main

import (
	"fmt"
	"sort"
)

func traceFrameStep(samples []TraceSample) int {
	deltas := []int{}
	for i := 1; i < len(samples); i++ {
		previous, current := samples[i-1], samples[i]
		if previous.Frame > 0 && current.Frame > previous.Frame && current.Time >= previous.Time {
			deltas = append(deltas, current.Frame-previous.Frame)
		}
	}
	if len(deltas) == 0 {
		return 0
	}
	sort.Ints(deltas)
	return deltas[(len(deltas)-1)/2]
}
func traceConnect(previous, current TraceSample, frameStep int) bool {
	if current.Distance < previous.Distance || current.Time < previous.Time {
		return false
	}
	if previous.Frame > 0 && current.Frame > previous.Frame && frameStep > 0 {
		return current.Frame-previous.Frame <= 3*frameStep
	}
	return current.Time-previous.Time <= .3
}
func traceResolution(samples []TraceSample) string {
	if len(samples) < 2 {
		return ""
	}
	deltas := []float64{}
	for i := 1; i < len(samples); i++ {
		dt := samples[i].Time - samples[i-1].Time
		if dt > 0 {
			deltas = append(deltas, dt)
		}
	}
	if len(deltas) == 0 {
		return ""
	}
	sort.Float64s(deltas)
	if deltas[(len(deltas)-1)/2] > .3 {
		zeroPedals := true
		for _, s := range samples {
			if s.Throttle != 0 || s.Brake != 0 {
				zeroPedals = false
				break
			}
		}
		if zeroPedals {
			return fmt.Sprintf("Lower-resolution trace: %d samples. Throttle/brake were reported as zero throughout", len(samples))
		}
		return fmt.Sprintf("Lower-resolution trace: %d samples; fine throttle/brake changes may be missed", len(samples))
	}
	return ""
}
