package loadgen

import (
	"time"
)

// Profile is the resolved load shape for one case run. Callers assemble it
// from a mode preset (in the evals package) and pass it to Generator.Run.
type Profile struct {
	// QPS is the target request rate the pacer will try to sustain. Must be
	// > 0.
	QPS float64
	// Concurrency is the number of worker goroutines executing the target
	// function. Sized to keep enough requests in flight for the target rate
	// at the target's expected latency (Little's law). Must be >= 1.
	Concurrency int
	// Duration is the measurement window: the wall-clock time samples are
	// collected over, excluding Warmup. Must be > 0.
	Duration time.Duration
	// Warmup is a leading window during which load is generated at the same
	// target rate but samples are discarded. Zero disables warmup.
	Warmup time.Duration
	// RequestTimeout bounds one call to the target function. Zero applies the
	// default (30s), and Run always caps it by the remaining window.
	RequestTimeout time.Duration
}

const defaultRequestTimeout = 30 * time.Second

func (p Profile) validate() error {
	if p.QPS <= 0 {
		return ErrInvalidProfile{Field: "QPS"}
	}
	if p.Concurrency < 1 {
		return ErrInvalidProfile{Field: "Concurrency"}
	}
	if p.Duration <= 0 {
		return ErrInvalidProfile{Field: "Duration"}
	}
	if p.Warmup < 0 {
		return ErrInvalidProfile{Field: "Warmup"}
	}
	if p.RequestTimeout < 0 {
		return ErrInvalidProfile{Field: "RequestTimeout"}
	}
	return nil
}

// resolvedRequestTimeout applies the default when RequestTimeout is zero.
func (p Profile) resolvedRequestTimeout() time.Duration {
	if p.RequestTimeout == 0 {
		return defaultRequestTimeout
	}
	return p.RequestTimeout
}
