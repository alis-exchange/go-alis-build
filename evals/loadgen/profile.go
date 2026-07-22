package loadgen

import (
	"fmt"
	"math"
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

// defaultRequestTimeout is applied when Profile.RequestTimeout is zero.
const defaultRequestTimeout = 30 * time.Second

// Validate checks profile invariants before a load window starts.
func (p Profile) Validate() error {
	return p.validate()
}

// validate checks profile invariants before a load window starts.
func (p Profile) validate() error {
	if p.Concurrency < 1 {
		return ErrInvalidProfile{Field: "Concurrency", Got: fmt.Sprintf("%d", p.Concurrency), Want: ">= 1"}
	}
	if p.Duration <= 0 {
		return ErrInvalidProfile{Field: "Duration", Got: p.Duration.String(), Want: "> 0"}
	}
	if p.Warmup < 0 {
		return ErrInvalidProfile{Field: "Warmup", Got: p.Warmup.String(), Want: ">= 0"}
	}
	if p.RequestTimeout < 0 {
		return ErrInvalidProfile{Field: "RequestTimeout", Got: p.RequestTimeout.String(), Want: ">= 0"}
	}
	if p.GracefulRampDown < 0 {
		return ErrInvalidProfile{Field: "GracefulRampDown", Got: p.GracefulRampDown.String(), Want: ">= 0"}
	}
	total := p.Warmup + p.Duration
	if len(p.QPSStages) == 0 {
		if err := validateRateTarget("QPS", p.QPS); err != nil {
			return err
		}
	} else if err := validateStages(p.QPSStages, "QPSStages", total, false); err != nil {
		return err
	}
	if len(p.ConcurrencyStages) > 0 {
		if err := validateStages(p.ConcurrencyStages, "ConcurrencyStages", total, true); err != nil {
			return err
		}
	}
	return nil
}

func validateRateTarget(field string, target float64) error {
	if target <= 0 || math.IsNaN(target) || math.IsInf(target, 0) {
		got := fmt.Sprintf("%v", target)
		if math.IsNaN(target) {
			got = "NaN"
		}
		return ErrInvalidProfile{Field: field, Got: got, Want: "> 0 finite"}
	}
	return nil
}

// validateStages ensures stage durations sum to want and each stage is positive.
func validateStages(stages []Stage, field string, want time.Duration, integralTargets bool) error {
	var sum time.Duration
	for i, s := range stages {
		if s.Duration <= 0 {
			return ErrInvalidProfile{Field: fmt.Sprintf("%s[%d].Duration", field, i), Got: s.Duration.String(), Want: "> 0"}
		}
		if err := validateStageTarget(field, i, s.Target, integralTargets); err != nil {
			return err
		}
		sum += s.Duration
	}
	if sum != want {
		return ErrInvalidProfile{Field: field, Got: sum.String(), Want: want.String()}
	}
	return nil
}

func validateStageTarget(field string, i int, target float64, integral bool) error {
	name := fmt.Sprintf("%s[%d].Target", field, i)
	if target <= 0 || math.IsNaN(target) || math.IsInf(target, 0) {
		got := fmt.Sprintf("%v", target)
		if math.IsNaN(target) {
			got = "NaN"
		}
		return ErrInvalidProfile{Field: name, Got: got, Want: "> 0 finite"}
	}
	if integral {
		trunc := math.Trunc(target)
		if target != trunc || target < 1 {
			return ErrInvalidProfile{Field: name, Got: fmt.Sprintf("%v", target), Want: "positive integer"}
		}
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

// MaxConcurrency returns the peak worker count for channel sizing and
// saturation warnings.
func (p Profile) MaxConcurrency() int {
	max := p.Concurrency
	for _, s := range p.ConcurrencyStages {
		if int(s.Target) > max {
			max = int(s.Target)
		}
	}
	return max
}

// initialConcurrency returns the worker count at window start for staged profiles.
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
