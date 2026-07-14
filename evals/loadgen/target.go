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
