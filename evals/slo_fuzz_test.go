package evals

import (
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/loadgen"
)

func builtinSLOsForFuzz() []SLO {
	return []SLO{
		SLOLatencyP50(100 * time.Millisecond),
		SLOLatencyP95(200 * time.Millisecond),
		SLOLatencyP99(500 * time.Millisecond),
		SLOErrorRate(0.05),
		SLOMinQPS(1),
		SLOStreamTTFB(100 * time.Millisecond),
		SLOMessagesPerSec(1),
	}
}

func FuzzAbortCheckMatchesEvaluateSLOs(f *testing.F) {
	f.Add(int64(100), int64(5), 50.0, 95.0, 99.0, 10.0, int64(10), int64(100), 80.0)
	f.Add(int64(0), int64(0), 0.0, 0.0, 0.0, 0.0, int64(0), int64(0), 0.0)

	f.Fuzz(func(t *testing.T, requestCount, errorCount int64, p50, p95, p99, actualQPS float64, streamCount, messagesTotal int64, ttfbP99 float64) {
		if requestCount < 0 || errorCount < 0 || streamCount < 0 || messagesTotal < 0 {
			return
		}
		if errorCount > requestCount {
			return
		}
		for _, v := range []float64{p50, p95, p99, actualQPS, ttfbP99} {
			if !(v >= 0 && v <= 1e9) {
				return
			}
		}
		if p50 > p95+1e-6 || p95 > p99+1e-6 {
			return
		}
		if streamCount > requestCount {
			return
		}

		m := &loadgen.Metrics{
			Duration:     time.Second,
			RequestCount: requestCount,
			ErrorCount:   errorCount,
			ActualQPS:    actualQPS,
			Latency: loadgen.LatencySummary{
				P50Ms: p50,
				P95Ms: p95,
				P99Ms: p99,
			},
		}
		if streamCount > 0 {
			m.Stream = &loadgen.StreamSummary{
				StreamCount:       streamCount,
				MessagesSentTotal: messagesTotal,
				TTFB:              loadgen.LatencySummary{P99Ms: ttfbP99},
			}
		}

		slos := builtinSLOsForFuzz()
		checks := evaluateSLOs(slos, m)
		check := abortCheckForSLOs(slos)
		if check == nil {
			t.Fatal("abortCheckForSLOs returned nil")
		}

		wantAbort := false
		if requestCount > 0 {
			for _, c := range checks {
				if c.Status != evalspb.Status_PASSED {
					wantAbort = true
					break
				}
			}
		}
		if got := check(m); got != wantAbort {
			t.Fatalf("abort=%v want=%v metrics=%+v checks=%+v", got, wantAbort, m, checks)
		}
	})
}
