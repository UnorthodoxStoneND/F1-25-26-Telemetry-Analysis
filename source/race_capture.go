package main

// STLG is the explicit red-light marker. The initial frame-zero LGOT may
// instead be formation start. Recorder and decoder share this state machine.
type RaceCaptureGate struct {
	Open         bool
	Frame        uint32
	Trigger      string
	Lights       int
	safety       int
	sawFormation bool
}

func raceSession(session int) bool { return session >= 15 && session <= 17 }
func (g *RaceCaptureGate) allow(b []byte, session int) bool {
	if !raceSession(session) {
		return true
	}
	if len(b) < 29 {
		return false
	}
	id, p := b[6], b[29:]
	frame := u32(b[23:])
	clock := f32(b[15:])
	code := ""
	if id == 3 && len(p) >= 4 {
		code = string(p[:4])
	}
	if id == 1 && len(p) > 124 {
		g.safety = int(p[124])
		g.sawFormation = g.sawFormation || g.safety == 3
	}
	if code == "FLBK" && len(p) >= 12 {
		if g.Open && u32(p[4:]) < g.Frame {
			g.Open = false
			g.Frame = 0
			g.Trigger = ""
			g.Lights = 0
		}
		return true
	}
	if g.Open {
		return true
	}
	open := func(trigger string) { g.Open = true; g.Frame = frame; g.Trigger = trigger }
	if code == "STLG" && len(p) >= 5 && p[4] >= 1 && p[4] <= 5 {
		open("start_lights")
		g.Lights = int(p[4])
		return true
	}
	if code == "LGOT" && frame > 0 && clock <= 1 && g.sawFormation && g.safety != 3 {
		open("lights_out_fallback")
		return true
	}
	if id == 2 && g.safety != 3 {
		count := 22
		if u16(b) == 2026 {
			count = 24
		}
		if len(p) >= count*57 {
			for i := 0; i < count; i++ {
				r := p[i*57:]
				if r[33] > 0 && u32(r[4:]) > 0 && (r[44] == 1 || r[44] == 4) {
					open("timed_lap_fallback")
					return true
				}
			}
		}
	}
	if id == 8 && len(p) >= 1 && p[0] > 0 && p[0] <= 24 && len(p) >= 1+int(p[0])*46 {
		for i := 0; i < int(p[0]); i++ {
			if p[1+i*46+1] > 0 {
				open("classification_fallback")
				return true
			}
		}
	}
	return id == 1 || id == 4 || id == 5 || id == 7 || code == "SEND"
}
