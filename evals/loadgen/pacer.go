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
	interval := float64(time.Second) / p.Freq
	target := float64(sent+1) * interval
	if target > math.MaxInt64 {
		return 0, true
	}
	delta := time.Duration(target) - elapsed
	if delta < 0 {
		return 0, false
	}
	return delta, false
}

// StepStagePacer holds a constant rate for each stage duration (ghz step).
type StepStagePacer struct {
	Stages   []Stage
	Duration time.Duration
}

// Pace implements Pacer.
func (p StepStagePacer) Pace(elapsed time.Duration, sent uint64) (time.Duration, bool) {
	if elapsed >= p.Duration {
		return 0, true
	}
	expected := p.expectedHits(elapsed)
	if float64(sent) < expected {
		return 0, false
	}
	wait := p.timeUntilHit(elapsed, float64(sent)+1)
	return wait, false
}

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
type LinearStagePacer struct {
	Stages   []Stage
	Duration time.Duration
}

// Pace implements Pacer.
func (p LinearStagePacer) Pace(elapsed time.Duration, sent uint64) (time.Duration, bool) {
	if elapsed >= p.Duration {
		return 0, true
	}
	expected := p.expectedHits(elapsed)
	if float64(sent) < expected {
		return 0, false
	}
	wait := p.timeUntilHit(elapsed, float64(sent)+1)
	return wait, false
}

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

func (p StepStagePacer) timeUntilHit(from time.Duration, targetHits float64) time.Duration {
	return timeUntilIntegratedHits(from, p.Duration, targetHits, p.expectedHits)
}

func (p LinearStagePacer) timeUntilHit(from time.Duration, targetHits float64) time.Duration {
	return timeUntilIntegratedHits(from, p.Duration, targetHits, p.expectedHits)
}

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

// pacerForProfile selects the pacer for a resolved profile and total window.
func pacerForProfile(p Profile, total time.Duration) Pacer {
	if len(p.QPSStages) == 0 {
		return ConstantPacer{Freq: p.QPS, Duration: total}
	}
	if p.QPSStageLinear {
		return LinearStagePacer{Stages: p.QPSStages, Duration: total}
	}
	return StepStagePacer{Stages: p.QPSStages, Duration: total}
}
