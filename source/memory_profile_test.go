package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestProfileSavedAnalysisMemory(t *testing.T) {
	dir := os.Getenv("UDP_TELEMETRY_MEMORY_PROFILE")
	if dir == "" {
		t.Skip("optional read-only memory profiling")
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, path := range paths {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var meta RecordingMeta
		if json.Unmarshal(b, &meta) != nil || meta.State != "completed" || meta.Session < 1 {
			continue
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		var peak atomic.Uint64
		peak.Store(before.HeapAlloc)
		done := make(chan struct{})
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			tick := time.NewTicker(5 * time.Millisecond)
			defer tick.Stop()
			for {
				select {
				case <-done:
					return
				case <-tick.C:
					var s runtime.MemStats
					runtime.ReadMemStats(&s)
					if s.HeapAlloc > peak.Load() {
						peak.Store(s.HeapAlloc)
					}
				}
			}
		}()
		start := time.Now()
		var m *AnalysisModel
		switch os.Getenv("UDP_TELEMETRY_MEMORY_SCOPE") {
		case "overview":
			m, e = loadAnalysisScoped(context.Background(), dir, meta, analysisTraceScope{})
		case "lap":
			m, e = loadAnalysisScoped(context.Background(), dir, meta, analysisTraceScope{Lap: 1, Drivers: (1 << 24) - 1})
		default:
			m, e = loadAnalysis(dir, meta)
		}
		if e != nil {
			close(done)
			<-stopped
			t.Fatal(e)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		samples := 0
		for _, d := range m.Drivers {
			samples += len(d.Samples)
		}
		close(done)
		<-stopped
		t.Logf("session=%d packets=%d samples=%d live=%.1fMiB peak=%.1fMiB allocated=%.1fMiB elapsed=%s", meta.Session, m.Frames, samples, float64(after.HeapAlloc-before.HeapAlloc)/(1<<20), float64(peak.Load()-before.HeapAlloc)/(1<<20), float64(after.TotalAlloc-before.TotalAlloc)/(1<<20), time.Since(start).Round(time.Millisecond))
		runtime.KeepAlive(m)
	}
}
