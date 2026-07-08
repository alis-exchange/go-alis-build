package loadgen

import "time"

// Metrics is the aggregate outcome of one load window, computed only from
// samples that fell inside the measurement window (Warmup excluded).
type Metrics struct {
	// Duration is the wall-clock time covered by the measurement window.
	Duration time.Duration
	// RequestCount is the number of requests whose result was observed inside
	// the measurement window.
	RequestCount int64
	// ErrorCount is the number of measured requests whose target function
	// returned a non-nil error (or panicked).
	ErrorCount int64
	// ActualQPS is RequestCount / Duration.Seconds().
	ActualQPS float64
	// Latency summarises the per-request latency distribution inside the
	// measurement window, in milliseconds.
	Latency LatencySummary
	// ErrorsByCode groups errors by their canonical gRPC status code name
	// (for example "UNAVAILABLE"). Non-gRPC errors are grouped under
	// "UNKNOWN". Values sum to ErrorCount.
	ErrorsByCode map[string]int64
}

// LatencySummary holds per-request latency percentiles and extremes in
// milliseconds. All fields are zero when no samples were collected.
type LatencySummary struct {
	// P50Ms is the median (50th percentile) request latency.
	P50Ms float64
	// P95Ms is the 95th percentile request latency.
	P95Ms float64
	// P99Ms is the 99th percentile request latency — the usual tail-latency
	// guardrail.
	P99Ms float64
	// MinMs is the fastest observed request.
	MinMs float64
	// MeanMs is the arithmetic mean over all measured requests.
	MeanMs float64
	// MaxMs is the slowest observed request.
	MaxMs float64
}
