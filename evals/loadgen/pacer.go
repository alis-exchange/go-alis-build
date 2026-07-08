package loadgen

import (
	"math"
	"time"
)

// Pacer decides when the next request should be sent. It is a small pure-math
// interface so the scheduling policy can evolve (step, ramp, etc.) without
// touching worker or aggregator code.
//
// Pace is called by the pacer goroutine with the elapsed wall-clock time
// since the window began and the total number of requests already scheduled.
// It returns how long to wait before the next send, and whether the window
// is complete.
//
// The math for the constant pacer is taken from vegeta/ghz: schedule request
// number N to fire at elapsed = N / rate. If we're behind schedule we fire
// immediately; otherwise we sleep until the absolute offset — which prevents
// scheduling error from accumulating across a long window.
type Pacer interface {
	Pace(elapsed time.Duration, sent uint64) (wait time.Duration, stop bool)
}

// ConstantPacer paces requests at a fixed rate for a fixed duration.
type ConstantPacer struct {
	// Freq is the target requests per second.
	Freq float64
	// Duration is the total scheduling window; Pace returns stop=true once
	// elapsed reaches Duration.
	Duration time.Duration
}

// Pace implements Pacer.
func (p ConstantPacer) Pace(elapsed time.Duration, sent uint64) (time.Duration, bool) {
	if elapsed >= p.Duration {
		return 0, true
	}
	if p.Freq <= 0 {
		return 0, true
	}
	// Ideal offset for the (sent+1)-th request in nanoseconds. Using float
	// arithmetic keeps sub-nanosecond fractional rates honest and matches
	// vegeta's approach.
	interval := float64(time.Second) / p.Freq
	target := float64(sent+1) * interval
	// Overflow guard: if the target offset overflows int64 we've been running
	// for something absurd — stop cleanly rather than panic.
	if target > math.MaxInt64 {
		return 0, true
	}
	delta := time.Duration(target) - elapsed
	if delta < 0 {
		return 0, false
	}
	return delta, false
}
