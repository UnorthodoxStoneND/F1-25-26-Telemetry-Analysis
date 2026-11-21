package main

// Temporary calculated cutoffs, explicitly selected by the user until the
// launcher exposes the actual advancement rule. Use the full racer field,
// never the nearby-driver slice. Q3/SQ3 and short/one-shot formats have no zone.
func qualifyingCutoff(f Frame) int {
	n := len(f.Cars)
	if n <= 10 || n > maxCars {
		return 0
	}
	switch f.SessionType {
	case 5, 10:
		return n - (n-10)/2
	case 6, 11:
		return 10
	}
	return 0
}
func qualifyingAtRisk(f Frame, c Car) bool {
	cutoff := qualifyingCutoff(f)
	if cutoff == 0 || c.Known&knownPosition == 0 || c.Position <= int32(cutoff) || c.Position > int32(len(f.Cars)) {
		return false
	}
	// A driver already out of the current session keeps the existing status
	// styling rather than appearing to be competing for an advancement place.
	if c.Known&knownResult != 0 {
		status := (c.Flags & resultMask) >> 8
		if status >= 4 {
			return false
		}
	}
	return true
}
