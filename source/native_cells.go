package main

// Results retain a numeric sort key even when the visible position is a medal.
func (a *App) podiumPosition(pos int) any {
	if pos >= 1 && pos <= 3 {
		return M{"text": []string{"🥇", "🥈", "🥉"}[pos-1], "sort": pos}
	}
	return knownInt(pos)
}

// The actual compound supplies the label; the game's visual compound supplies
// its weekend-specific dry colour. Unknown colours remain uncoloured text.
func tyreCell(c Car) M {
	if !c.HasStatus || compound(c.Compound) == "—" {
		return M{"text": "—", "tone": "muted"}
	}
	label := compound(c.Compound)
	cell := M{"text": label, "sort": label}
	colours := map[string]string{"red": "#FF4A5B", "yellow": "#FFD53F", "white": "#EEF2FA", "green": "#41DA8B", "blue": "#4B9FFF"}
	if value := colours[tyreColour(c)]; value != "" {
		cell["edit"] = M{"colour": M{"value": value, "disabled": true}}
	}
	return cell
}
