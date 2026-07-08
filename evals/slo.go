package evals

import (
	"fmt"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadgen"
)

// SLO is one pass/fail threshold check evaluated against the aggregate
// metrics of a load window. Every configured SLO produces one SloCheckResult
// per case run — passed and failed alike — so consumers see headroom on
// passed checks, not just breaches.
type SLO struct {
	id      string
	unit    string
	limit   float64
	extract func(*loadgen.Metrics) float64
	// pass reports whether observed satisfies the limit. Encoded per-SLO
	// because latency SLOs are upper bounds while SLOMinQPS is a lower
	// bound.
	pass func(observed, limit float64) bool
	// failMessage formats a helpful triage message when pass returns false.
	failMessage func(observed, limit float64) string
}

// SLOLatencyP50 asserts that measured p50 latency stays at or below max.
func SLOLatencyP50(max time.Duration) SLO {
	return latencySLO("latency.p50_ms", max, func(m *loadgen.Metrics) float64 { return m.Latency.P50Ms })
}

// SLOLatencyP95 asserts that measured p95 latency stays at or below max.
func SLOLatencyP95(max time.Duration) SLO {
	return latencySLO("latency.p95_ms", max, func(m *loadgen.Metrics) float64 { return m.Latency.P95Ms })
}

// SLOLatencyP99 asserts that measured p99 latency stays at or below max.
func SLOLatencyP99(max time.Duration) SLO {
	return latencySLO("latency.p99_ms", max, func(m *loadgen.Metrics) float64 { return m.Latency.P99Ms })
}

func latencySLO(id string, max time.Duration, extract func(*loadgen.Metrics) float64) SLO {
	limitMs := float64(max) / float64(time.Millisecond)
	return SLO{
		id:      id,
		unit:    "ms",
		limit:   limitMs,
		extract: extract,
		pass:    func(observed, limit float64) bool { return observed <= limit },
		failMessage: func(observed, limit float64) string {
			return fmt.Sprintf("%.1fms exceeds limit %.1fms", observed, limit)
		},
	}
}

// SLOErrorRate asserts that the measured error rate stays at or below the
// given fraction (for example 0.01 for 1%). Observed and limit are recorded
// as percent so the wire values match human intuition.
func SLOErrorRate(maxFraction float64) SLO {
	limitPct := maxFraction * 100
	return SLO{
		id:   "error_rate",
		unit: "%",
		extract: func(m *loadgen.Metrics) float64 {
			if m.RequestCount == 0 {
				return 0
			}
			return float64(m.ErrorCount) / float64(m.RequestCount) * 100
		},
		limit: limitPct,
		pass:  func(observed, limit float64) bool { return observed <= limit },
		failMessage: func(observed, limit float64) string {
			return fmt.Sprintf("%.2f%% exceeds limit %.2f%%", observed, limit)
		},
	}
}

// SLOMinQPS asserts that observed throughput stays at or above min.
func SLOMinQPS(min float64) SLO {
	return SLO{
		id:      "actual_qps",
		unit:    "rps",
		extract: func(m *loadgen.Metrics) float64 { return m.ActualQPS },
		limit:   min,
		pass:    func(observed, limit float64) bool { return observed >= limit },
		failMessage: func(observed, limit float64) string {
			return fmt.Sprintf("%.1frps below floor %.1frps", observed, limit)
		},
	}
}

// evaluateSLOs runs every SLO against m and returns one SloCheckResult per
// SLO in declaration order.
func evaluateSLOs(slos []SLO, m *loadgen.Metrics) []execution.SloCheckResult {
	if len(slos) == 0 || m == nil {
		return nil
	}
	out := make([]execution.SloCheckResult, 0, len(slos))
	for _, s := range slos {
		observed := s.extract(m)
		passed := s.pass(observed, s.limit)
		status := evalspb.Status_PASSED
		msg := ""
		if !passed {
			status = evalspb.Status_FAILED
			msg = s.failMessage(observed, s.limit)
		}
		out = append(out, execution.SloCheckResult{
			ID:       s.id,
			Status:   status,
			Message:  msg,
			Observed: observed,
			Limit:    s.limit,
			Unit:     s.unit,
		})
	}
	return out
}
