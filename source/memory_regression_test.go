package main

import (
	"context"
	"runtime"
	"testing"
)

// Check retained telemetry rather than runner-wide RAM, which also includes
// the compiler, OS, graphics driver and unrelated GitHub Actions processes.
func TestAnalysisRetentionBudget(t *testing.T) {
	dir, meta := scopedJournal(t, 2026)
	const budget = 8 << 20
	for _, scope := range []analysisTraceScope{{}, {Lap: 1, Drivers: (1 << 24) - 1}} {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		model, err := loadAnalysisScoped(context.Background(), dir, meta, scope)
		if err != nil {
			t.Fatal(err)
		}
		samples := 0
		for driver, data := range model.Drivers {
			for _, sample := range data.Samples {
				if !scope.captures(driver, sample.Lap) {
					t.Fatal("Unrequested lap/driver retained")
				}
				samples++
			}
		}
		if !scope.hasTraces() && samples != 0 {
			t.Fatal("Overview retained driving traces")
		}
		if scope.hasTraces() && samples == 0 {
			t.Fatal("Trace budget test had no driving samples")
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		if after.HeapAlloc > before.HeapAlloc && after.HeapAlloc-before.HeapAlloc > budget {
			t.Fatalf("Fixed fixture retained %d bytes; budget %d", after.HeapAlloc-before.HeapAlloc, budget)
		}
		model.releaseUnusedTraces(analysisTraceScope{})
		for _, data := range model.Drivers {
			if data.Samples != nil {
				t.Fatal("Leaving Lap Analysis retained trace buffers")
			}
		}
		runtime.KeepAlive(model)
	}
}
