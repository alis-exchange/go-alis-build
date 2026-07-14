package loadgen

import (
	"fmt"
	"time"
)

// Stage is one step in a staged load profile. Target is queries per second
// when used in QPSStages, or worker count when used in ConcurrencyStages.
type Stage struct {
	Duration time.Duration
	Target   float64
}

// AbortCheck optionally evaluates partial metrics during a run. When it
// returns true the generator cancels the window early.
type AbortCheck func(*Metrics) bool

// Profile is the resolved load shape for one case run. Callers assemble it
// from a mode preset (in the evals package) and pass it to Generator.Run.
type Profile struct {
	// QPS is the target request rate the pacer will try to sustain when
	// QPSStages is empty. Must be > 0 unless QPSStages is set.
	QPS float64
	// Concurrency is the number of worker goroutines when ConcurrencyStages is
	// empty. When ConcurrencyStages is set, this is the fallback maximum used
	// for channel sizing. Must be >= 1.
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
	// QPSStages defines a piecewise load shape over Warmup+Duration. When
	// non-empty, stage durations must sum to Warmup+Duration and override the
	// constant QPS pacer.
	QPSStages []Stage
	// QPSStageLinear selects linear interpolation between consecutive QPS stage
	// targets (ghz-style 1s micro-steps). When false, each stage holds its
	// target rate for its full duration.
	QPSStageLinear bool
	// ConcurrencyStages scales the worker pool over Warmup+Duration. When
	// non-empty, stage durations must sum to Warmup+Duration.
	ConcurrencyStages []Stage
	// GracefulRampDown allows in-flight requests to complete after the
	// measurement window ends. Samples scheduled at or after the measurement
	// boundary are excluded from aggregates.
	GracefulRampDown time.Duration
	// AbortCheck cancels the window early when it returns true on a partial
	// metrics snapshot (typically every 2s).
	AbortCheck AbortCheck
}

const defaultRequestTimeout = 30 * time.Second

func (p Profile) validate() error {
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
	if p.GracefulRampDown < 0 {
		return ErrInvalidProfile{Field: "GracefulRampDown"}
	}
	total := p.Warmup + p.Duration
	if len(p.QPSStages) == 0 {
		if p.QPS <= 0 {
			return ErrInvalidProfile{Field: "QPS"}
		}
	} else if err := validateStages(p.QPSStages, "QPSStages", total); err != nil {
		return err
	}
	if len(p.ConcurrencyStages) > 0 {
		if err := validateStages(p.ConcurrencyStages, "ConcurrencyStages", total); err != nil {
			return err
		}
	}
	return nil
}

func validateStages(stages []Stage, field string, want time.Duration) error {
	var sum time.Duration
	for i, s := range stages {
		if s.Duration <= 0 {
			return ErrInvalidProfile{Field: fmt.Sprintf("%s[%d].Duration", field, i)}
		}
		if s.Target <= 0 {
			return ErrInvalidProfile{Field: fmt.Sprintf("%s[%d].Target", field, i)}
		}
		sum += s.Duration
	}
	if sum != want {
		return ErrInvalidProfile{Field: field}
	}
	return nil
}

// EffectiveQPS returns the peak configured rate for saturation warnings.
func (p Profile) EffectiveQPS() float64 {
	if len(p.QPSStages) == 0 {
		return p.QPS
	}
	max := 0.0
	for _, s := range p.QPSStages {
		if s.Target > max {
			max = s.Target
		}
	}
	return max
}

func (p Profile) MaxConcurrency() int {
	max := p.Concurrency
	for _, s := range p.ConcurrencyStages {
		if int(s.Target) > max {
			max = int(s.Target)
		}
	}
	return max
}

func (p Profile) initialConcurrency() int {
	if len(p.ConcurrencyStages) == 0 {
		return p.Concurrency
	}
	return int(p.ConcurrencyStages[0].Target)
}

// resolvedRequestTimeout applies the default when RequestTimeout is zero.
func (p Profile) resolvedRequestTimeout() time.Duration {
	if p.RequestTimeout == 0 {
		return defaultRequestTimeout
	}
	return p.RequestTimeout
}
