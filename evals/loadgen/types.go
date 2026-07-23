package loadgen

import (
	"context"
	"time"
)

// CallData is per-request context passed to a load target.
type CallData struct {
	// RequestNumber is the 1-based index of this request in the window.
	RequestNumber uint64
	// WorkerID identifies the worker goroutine executing this request.
	WorkerID int
	// Data holds the resolved payload from round-robin or a DataProvider.
	Data any
}

// TargetResult separates transport failures from semantic check failures.
// Use TransportErr for RPC/transport problems; use CheckErr for assertions
// that passed transport but failed a semantic predicate (for example score
// thresholds). Check failures roll up to a failed case via a synthetic
// "checks" SloCheckResult when no explicit SLO covers them.
type TargetResult struct {
	TransportErr error
	CheckErr     error
	// Stream holds optional streaming RPC timing for one invocation.
	Stream *StreamSample
}

// StreamSample captures timing from one streaming RPC invocation.
type StreamSample struct {
	// SendDuration spans stream open through the last successful Send on a
	// client-streaming RPC.
	SendDuration    time.Duration
	ResponseLatency time.Duration
	TotalDuration   time.Duration
	MessagesSent    int
}

// ResultTarget executes exactly one load request.
type ResultTarget func(context.Context, CallData) TargetResult

// TransportTarget adapts a transport-only function to a [ResultTarget].
func TransportTarget(fn func(context.Context) error) ResultTarget {
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, _ CallData) TargetResult {
		return TargetResult{TransportErr: fn(ctx)}
	}
}

// Metrics is the aggregate outcome of one load window, computed only from
// samples that fell inside the measurement window (Warmup excluded).
type Metrics struct {
	// Duration is the wall-clock time covered by the measurement window.
	Duration time.Duration
	// RequestCount is the number of requests whose result was observed inside
	// the measurement window.
	RequestCount int64
	// ErrorCount is transport failures only (including recovered panics as
	// INTERNAL). Semantic check failures are counted in CheckFailedCount.
	ErrorCount int64
	// ActualQPS is RequestCount / Duration.Seconds().
	ActualQPS float64
	// Latency summarises the per-request latency distribution in milliseconds.
	Latency LatencySummary
	// CheckPassedCount is semantic assertions that passed.
	CheckPassedCount int64
	// CheckFailedCount is semantic assertion failures.
	CheckFailedCount int64
	// ErrorsByCode groups errors by canonical gRPC status code name.
	ErrorsByCode map[string]int64
	// DroppedCount is scheduled ticks that were not dispatched.
	DroppedCount int64
	// Stream holds aggregate streaming metrics.
	Stream *StreamSummary
	// MeasurementStart is the inclusive start of the measurement window.
	MeasurementStart time.Time
	// MeasurementEnd is the exclusive end of the measurement window.
	MeasurementEnd time.Time
}

// StreamSummary aggregates streaming RPC metrics for one load window.
type StreamSummary struct {
	StreamCount       int64
	MessagesSentTotal int64
	// TTFB aggregates [StreamSample.SendDuration].
	TTFB            LatencySummary
	ResponseLatency LatencySummary
	TotalDuration   LatencySummary
}

// LatencySummary holds per-request latency percentiles and extremes in
// milliseconds. All fields are zero when no samples were collected.
type LatencySummary struct {
	// P50Ms is the median request latency.
	P50Ms float64
	// P95Ms is the 95th percentile request latency.
	P95Ms float64
	// P99Ms is the 99th percentile request latency.
	P99Ms float64
	// MinMs is the fastest observed request.
	MinMs float64
	// MeanMs is the arithmetic mean.
	MeanMs float64
	// MaxMs is the slowest observed request.
	MaxMs float64
}
