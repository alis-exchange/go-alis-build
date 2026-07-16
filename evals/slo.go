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
	// id is the wire check id (for example "latency.p95_ms").
	id string
	// unit is the observed/limit unit on the wire (ms, %, rps, msg/s).
	unit string
	// limit is the threshold in unit; upper bound for latency/error SLOs, lower bound for QPS.
	limit float64
	// extract projects aggregate metrics to the observed value.
	extract func(*loadgen.Metrics) float64
	// pass reports whether observed satisfies the limit. Latency SLOs use upper
	// bounds; SLOMinQPS uses a lower bound.
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

// latencySLO builds an upper-bound latency SLO from a percentile extractor.
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

// SLOStreamTTFB asserts that measured stream send-duration p99 stays at or
// below max. Targets must populate [loadgen.Metrics.Stream] via
// [StreamSample.SendDuration] (for example through [ClientStreamTargetResult]).
// The value maps to the wire StreamSummary.ttfb field. Fails when no stream
// samples were recorded.
func SLOStreamTTFB(max time.Duration) SLO {
	limitMs := float64(max) / float64(time.Millisecond)
	return SLO{
		id:    "stream.ttfb_p99_ms",
		unit:  "ms",
		limit: limitMs,
		extract: func(m *loadgen.Metrics) float64 {
			if m.Stream == nil || m.Stream.StreamCount == 0 {
				return -1
			}
			return m.Stream.TTFB.P99Ms
		},
		pass: func(observed, limit float64) bool {
			if observed < 0 {
				return false
			}
			return observed <= limit
		},
		failMessage: func(observed, limit float64) string {
			if observed < 0 {
				return "no stream samples recorded"
			}
			return fmt.Sprintf("%.1fms exceeds limit %.1fms", observed, limit)
		},
	}
}

// SLOMessagesPerSec asserts that aggregate outbound stream message rate stays
// at or above min across the measurement window.
func SLOMessagesPerSec(min float64) SLO {
	return SLO{
		id:   "stream.messages_per_sec",
		unit: "msg/s",
		extract: func(m *loadgen.Metrics) float64 {
			if m.Stream == nil || m.Duration <= 0 {
				return 0
			}
			return float64(m.Stream.MessagesSentTotal) / m.Duration.Seconds()
		},
		limit: min,
		pass:  func(observed, limit float64) bool { return observed >= limit },
		failMessage: func(observed, limit float64) string {
			return fmt.Sprintf("%.1fmsg/s below floor %.1fmsg/s", observed, limit)
		},
	}
}

// abortCheckForSLOs returns a partial-metrics check that reports true when
// any configured SLO would fail. Zero request windows are ignored so abort
// does not fire before the first measured sample.
func abortCheckForSLOs(slos []SLO) loadgen.AbortCheck {
	if len(slos) == 0 {
		return nil
	}
	return func(m *loadgen.Metrics) bool {
		if m == nil || m.RequestCount == 0 {
			return false
		}
		for _, s := range slos {
			observed := s.extract(m)
			if !s.pass(observed, s.limit) {
				return true
			}
		}
		return false
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
