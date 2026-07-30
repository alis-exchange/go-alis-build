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
// The math for the constant pacer follows vegeta/ghz — schedule against an
// absolute offset so scheduling error cannot accumulate across a long window —
// with one deliberate divergence: request number N fires at elapsed = N / rate
// (slot 0 at t=0), not (N+1)/rate. vegeta delays the first request by one full
// interval, which is noise at web-service rates but pure dead wall clock for
// slow-iteration load tests: at QPS = 1/expected-iteration, a case would idle
// one whole expected iteration before doing any work. If we're behind schedule
// we fire immediately; otherwise we sleep until the absolute offset.
//
// A corollary of the slot-0 rule: every pacer in this package dispatches
// request 0 at t=0 unconditionally, regardless of the rate curve — even a
// staged profile with a near-idle lead-in stage fires its first request
// immediately. Only requests 1..N wait for the integrated rate to catch up.
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
	interval := float64(time.Second) / p.Freq
	// Slot 0 fires at t=0; slot N at N/rate. See the interface comment for
	// why this diverges from vegeta's (N+1)/rate.
	target := float64(sent) * interval
	// A Freq small enough to overflow interval to +Inf makes target 0×Inf=NaN
	// at sent==0; NaN compares false against every bound, so without this
	// guard it would flow into an implementation-defined time.Duration
	// conversion below. Such a rate schedules no slot inside any real window —
	// stop cleanly, as the pre-slot-0 schedule did via its (sent+1) overflow.
	if math.IsNaN(target) || target > math.MaxInt64 {
		return 0, true
	}
	delta := time.Duration(target) - elapsed
	if delta < 0 {
		return 0, false
	}
	return delta, false
}

// StepStagePacer holds a constant rate for each stage duration (ghz step).
//
// Like every pacer in this package it follows the slot-0 schedule: request 0
// fires at t=0 regardless of the first stage's rate (see the [Pacer] comment),
// so a near-idle lead-in stage does not delay the first dispatch.
type StepStagePacer struct {
	Stages   []Stage
	Duration time.Duration
}

// Pace implements Pacer.
func (p StepStagePacer) Pace(elapsed time.Duration, sent uint64) (time.Duration, bool) {
	if elapsed >= p.Duration {
		return 0, true
	}
	// Request N fires when the integrated rate reaches N (slot 0 at t=0),
	// matching ConstantPacer's slot-0 schedule.
	expected := p.expectedHits(elapsed)
	if float64(sent) <= expected {
		return 0, false
	}
	wait := p.timeUntilHit(elapsed, float64(sent))
	return wait, false
}

// expectedHits returns the cumulative request count the step pacer should have
// scheduled by elapsed, integrating constant rate over each stage segment.
func (p StepStagePacer) expectedHits(elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	cap := elapsed
	if cap > p.Duration {
		cap = p.Duration
	}
	var hits float64
	var used time.Duration
	for _, s := range p.Stages {
		if used >= cap {
			break
		}
		seg := s.Duration
		if used+seg > cap {
			seg = cap - used
		}
		hits += s.Target * seg.Seconds()
		used += seg
	}
	return hits
}

// LinearStagePacer linearly interpolates between consecutive stage targets
// over each stage duration (ghz line-style ramps at stage boundaries).
//
// Like every pacer in this package it follows the slot-0 schedule: request 0
// fires at t=0 regardless of the ramp's starting rate (see the [Pacer]
// comment); only later requests wait for the integrated rate curve.
type LinearStagePacer struct {
	Stages   []Stage
	Duration time.Duration
}

// Pace implements Pacer.
func (p LinearStagePacer) Pace(elapsed time.Duration, sent uint64) (time.Duration, bool) {
	if elapsed >= p.Duration {
		return 0, true
	}
	// Slot-0 schedule; see StepStagePacer.Pace.
	expected := p.expectedHits(elapsed)
	if float64(sent) <= expected {
		return 0, false
	}
	wait := p.timeUntilHit(elapsed, float64(sent))
	return wait, false
}

// expectedHits returns the cumulative request count the linear pacer should have
// scheduled by elapsed, integrating the trapezoidal rate curve within each stage.
func (p LinearStagePacer) expectedHits(elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	cap := elapsed
	if cap > p.Duration {
		cap = p.Duration
	}
	var hits float64
	offset := time.Duration(0)
	for i, s := range p.Stages {
		if offset >= cap {
			break
		}
		segDur := s.Duration
		segElapsed := segDur
		if offset+segDur > cap {
			segElapsed = cap - offset
		}
		end := s.Target
		if i+1 < len(p.Stages) {
			end = p.Stages[i+1].Target
		}
		hits += linearSegmentHits(s.Target, end, segDur, segElapsed)
		offset += segElapsed
	}
	return hits
}

// linearSegmentHits integrates request count for one linear ramp segment from
// start to end over stageDur when elapsed time has passed within the segment.
func linearSegmentHits(start, end float64, stageDur, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	d := stageDur.Seconds()
	e := elapsed.Seconds()
	if e > d {
		e = d
	}
	if d == 0 {
		return 0
	}
	return start*e + (end-start)*e*e/(2*d)
}

// timeUntilHit returns how long to wait from from until the step pacer reaches
// targetHits cumulative scheduled requests.
func (p StepStagePacer) timeUntilHit(from time.Duration, targetHits float64) time.Duration {
	return timeUntilIntegratedHits(from, p.Duration, targetHits, p.expectedHits)
}

// timeUntilHit returns how long to wait from from until the linear pacer reaches
// targetHits cumulative scheduled requests.
func (p LinearStagePacer) timeUntilHit(from time.Duration, targetHits float64) time.Duration {
	return timeUntilIntegratedHits(from, p.Duration, targetHits, p.expectedHits)
}

// timeUntilIntegratedHits binary-searches elapsed time until integrate(t) reaches
// targetHits, shared by step and linear staged pacers.
func timeUntilIntegratedHits(from, total time.Duration, targetHits float64, integrate func(time.Duration) float64) time.Duration {
	if integrate(total) <= targetHits {
		return total - from
	}
	lo, hi := from, total
	for hi-lo > time.Millisecond {
		mid := lo + (hi-lo)/2
		if integrate(mid) < targetHits {
			lo = mid
		} else {
			hi = mid
		}
	}
	wait := hi - from
	if wait < 0 {
		return 0
	}
	return wait
}

// pacerForProfile selects constant, step, or linear staged pacing for the
// resolved profile over the full Warmup+Duration window.
func pacerForProfile(p Profile, total time.Duration) Pacer {
	if len(p.QPSStages) == 0 {
		return ConstantPacer{Freq: p.QPS, Duration: total}
	}
	if p.QPSStageLinear {
		return LinearStagePacer{Stages: p.QPSStages, Duration: total}
	}
	return StepStagePacer{Stages: p.QPSStages, Duration: total}
}
