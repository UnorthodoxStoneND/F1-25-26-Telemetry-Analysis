package main

import "fmt"

func sessionSummary(f Frame) string {
	label := sessionLabel(f)
	if label == "" {
		return ""
	}
	if f.State&sessionTimer != 0 && f.Progress <= 86400 {
		return fmt.Sprintf("%s · %02d:%02d remaining", label, f.Progress/60, f.Progress%60)
	}
	if f.State&sessionLaps != 0 {
		current, total := f.Progress&65535, f.Progress>>16
		if current > 0 && current <= total && total <= 1000 {
			return fmt.Sprintf("%s · Lap %d/%d", label, current, total)
		}
	}
	return label
}

func sessionLabel(f Frame) string {
	if f.State&active == 0 {
		return ""
	}
	labels := [...]string{"", "Practice 1", "Practice 2", "Practice 3", "Short Practice",
		"Qualifying 1", "Qualifying 2", "Qualifying 3", "Short Qualifying", "One-Shot Qualifying",
		"Sprint Qualifying 1", "Sprint Qualifying 2", "Sprint Qualifying 3", "Short Sprint Qualifying", "One-Shot Sprint Qualifying"}
	if f.SessionType > 0 && f.SessionType < uint32(len(labels)) {
		return labels[f.SessionType]
	}
	switch f.SessionType {
	case 15:
		switch f.SessionKind {
		case 3:
			return "Race"
		case 4:
			return "Sprint"
		default:
			return "Race / Sprint (unconfirmed)"
		}
	case 16, 17:
		return "Race"
	case 18:
		return "Time Trial"
	case 0:
		// Context can still identify the family when the numeric binding is unavailable.
		families := [...]string{"Unknown", "Practice", "Qualifying", "Race", "Sprint", "Time Trial"}
		if f.SessionKind < uint32(len(families)) {
			return families[f.SessionKind]
		}
	}
	return "Unknown"
}
