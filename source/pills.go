package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// Small embedded glyphs keep the Windows helper self-contained. The capsule
// background has real alpha; colours are independent of the launcher's theme.
var glyphs = map[rune][]string{
	'C': {"01110", "10001", "10000", "10000", "10000", "10001", "01110"},
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'I': {"11111", "00100", "00100", "00100", "00100", "00100", "11111"},
	'N': {"10001", "11001", "11001", "10101", "10011", "10011", "10001"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "11011", "10001"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
}

func makePills(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	colours := map[string]color.NRGBA{"red": {255, 74, 91, 255}, "yellow": {255, 213, 63, 255}, "white": {238, 242, 250, 255}, "green": {65, 218, 139, 255}, "blue": {75, 159, 255, 255}}
	for _, label := range []string{"C0", "C1", "C2", "C3", "C4", "C5", "C6", "INT", "WET"} {
		for name, col := range colours {
			img := image.NewNRGBA(image.Rect(0, 0, 174, 78))
			for y := 0; y < 78; y++ {
				for x := 0; x < 174; x++ {
					cx := float64(x)
					if x < 39 {
						cx = 39
					}
					if x > 134 {
						cx = 134
					}
					dx, dy := float64(x)-cx, float64(y)-38.5
					if dx*dx+dy*dy < 38*38 {
						bg := col
						bg.A = 48
						img.SetNRGBA(x, y, bg)
					}
				}
			}
			scale := 6
			start := (174 - (len(label)*6-1)*scale) / 2
			for i, ch := range label {
				for y, row := range glyphs[ch] {
					for x, pixel := range row {
						if pixel == '1' {
							for yy := 0; yy < scale; yy++ {
								for xx := 0; xx < scale; xx++ {
									img.SetNRGBA(start+i*6*scale+x*scale+xx, 18+y*scale+yy, col)
								}
							}
						}
					}
				}
			}
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%s.png", label, name)))
			if err != nil {
				return err
			}
			err = png.Encode(f, img)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	return nil
}
