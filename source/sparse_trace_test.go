package main

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSparseContinuousFramesProduceVisibleTrace(t *testing.T) {
	sparse := []TraceSample{}
	for i := 0; i < 57; i++ {
		sparse = append(sparse, TraceSample{Time: 680 + float64(i)*1.4, Frame: 13629 + i, Distance: 20 + float64(i)*90, Speed: 200 + float64(i%5)*10, Throttle: 80, Brake: 10, RPM: 11000, Gear: 5})
	}
	step := traceFrameStep(sparse)
	if step != 1 || !traceConnect(sparse[0], sparse[1], step) {
		t.Fatal("consecutive frames were treated as missing data")
	}
	missing := sparse[1]
	missing.Frame = sparse[0].Frame + 10
	if traceConnect(sparse[0], missing, step) {
		t.Fatal("missing frame gap was bridged")
	}
	reversed := sparse[1]
	reversed.Distance = 0
	if traceConnect(sparse[0], reversed, step) {
		t.Fatal("distance reset bridged")
	}
	if !strings.Contains(traceResolution(sparse), "57 samples") {
		t.Fatal("sparse resolution not labelled")
	}
	zeroPedals := append([]TraceSample(nil), sparse...)
	for i := range zeroPedals {
		zeroPedals[i].Throttle, zeroPedals[i].Brake = 0, 0
	}
	if !strings.Contains(traceResolution(zeroPedals), "reported as zero") {
		t.Fatal("zero pedal channels not explained")
	}
	path := filepath.Join(t.TempDir(), "sparse.png")
	if e := renderTrace(path, nil, sparse, 0); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	im, e := png.Decode(f)
	if e != nil {
		t.Fatal(e)
	}
	orange := 0
	for y := 60; y <= 211; y++ {
		for x := 120; x <= 1550; x++ {
			if color.RGBAModel.Convert(im.At(x, y)).(color.RGBA) == (color.RGBA{255, 167, 82, 255}) {
				orange++
			}
		}
	}
	if orange < 1000 {
		t.Fatal("sparse comparison remains isolated dots", orange)
	}
}
func TestRegularAndFallbackTraceGaps(t *testing.T) {
	normal := []TraceSample{{Time: 1, Frame: 10, Distance: 1}, {Time: 1.05, Frame: 13, Distance: 4}, {Time: 1.1, Frame: 16, Distance: 8}, {Time: 1.15, Frame: 19, Distance: 12}}
	step := traceFrameStep(normal)
	if step != 3 || !traceConnect(normal[0], normal[1], step) || traceResolution(normal) != "" {
		t.Fatal("normal cadence changed")
	}
	if traceConnect(TraceSample{Time: 1, Distance: 1}, TraceSample{Time: 2, Distance: 2}, 0) {
		t.Fatal("unknown-frame time gap bridged")
	}
}
