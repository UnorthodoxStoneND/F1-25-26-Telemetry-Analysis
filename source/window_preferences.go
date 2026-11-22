package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

type windowRect struct{ X, Y, W, H int }
type windowPlacement struct {
	Version   int        `json:"version"`
	Bounds    windowRect `json:"bounds"`
	Maximized bool       `json:"maximized"`
}

func validWindowRect(r windowRect) bool {
	return r.W >= 100 && r.H >= 100 && r.W <= 32768 && r.H <= 32768 && r.X >= -1000000 && r.X <= 1000000 && r.Y >= -1000000 && r.Y <= 1000000
}
func windowPreferencePath(data string) string {
	if data == "" {
		return ""
	}
	return filepath.Join(data, "retro-window.json")
}
func loadWindowPlacement(path string) (windowPlacement, bool) {
	if path == "" {
		return windowPlacement{}, false
	}
	file, err := os.Open(path)
	if err != nil {
		return windowPlacement{}, false
	}
	defer file.Close()
	var p windowPlacement
	if json.NewDecoder(io.LimitReader(file, 4096)).Decode(&p) != nil || p.Version != 1 || !validWindowRect(p.Bounds) {
		return windowPlacement{}, false
	}
	return p, true
}
func saveWindowPlacement(path string, p windowPlacement) error {
	if path == "" || !validWindowRect(p.Bounds) {
		return nil
	}
	p.Version = 1
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".retro-window-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(raw)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	return err
}
func intersectionArea(a, b windowRect) int {
	w := min(a.X+a.W, b.X+b.W) - max(a.X, b.X)
	h := min(a.Y+a.H, b.Y+b.H) - max(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}
func fitWindowPlacement(saved windowPlacement, exists bool, work []windowRect, preferred int) windowPlacement {
	if len(work) == 0 {
		return saved
	}
	if preferred < 0 || preferred >= len(work) {
		preferred = 0
	}
	target := preferred
	if exists {
		area := 0
		for i, monitor := range work {
			if hit := intersectionArea(saved.Bounds, monitor); hit > area {
				area = hit
				target = i
			}
		}
	} else {
		saved = windowPlacement{Version: 1, Bounds: windowRect{W: 1600, H: 920}}
	}
	r := work[target]
	b := saved.Bounds
	b.W = min(b.W, r.W)
	b.H = min(b.H, r.H)
	if !exists {
		b.X = r.X + (r.W-b.W)/2
		b.Y = r.Y + (r.H-b.H)/2
	}
	b.X = max(r.X, min(b.X, r.X+r.W-b.W))
	b.Y = max(r.Y, min(b.Y, r.Y+r.H-b.H))
	saved.Bounds = b
	return saved
}
