package loadgen

import (
	"testing"
	"time"
)

func TestConstantPacer_Pace(t *testing.T) {
	t.Parallel()

	p := ConstantPacer{Freq: 100, Duration: time.Second}
	// Slot 0 fires at t=0: no lead-in interval before the first request.
	if wait, stop := p.Pace(0, 0); stop || wait != 0 {
		t.Fatalf("start: wait=%v stop=%v, want 0/false", wait, stop)
	}
	// Behind schedule: send immediately.
	if wait, stop := p.Pace(50*time.Millisecond, 0); stop || wait != 0 {
		t.Fatalf("behind: wait=%v stop=%v, want 0/false", wait, stop)
	}
	// Slot 1 is scheduled at 10ms.
	if wait, stop := p.Pace(0, 1); stop || wait != 10*time.Millisecond {
		t.Fatalf("next slot: wait=%v stop=%v, want 10ms/false", wait, stop)
	}
	// On schedule: slot 1's offset has arrived, fire now.
	if wait, stop := p.Pace(10*time.Millisecond, 1); stop || wait != 0 {
		t.Fatalf("on schedule: wait=%v stop=%v, want 0/false", wait, stop)
	}
	// Window ended.
	if _, stop := p.Pace(time.Second, 100); !stop {
		t.Fatal("window end: stop=false, want true")
	}
	if _, stop := p.Pace(2*time.Second, 100); !stop {
		t.Fatal("past window: stop=false, want true")
	}
}

func TestConstantPacer_ZeroFreq(t *testing.T) {
	t.Parallel()

	p := ConstantPacer{Freq: 0, Duration: time.Second}
	if _, stop := p.Pace(0, 0); !stop {
		t.Fatal("zero freq: stop=false, want true")
	}
}

func TestStepStagePacer_holdsRatePerStage(t *testing.T) {
	t.Parallel()

	p := StepStagePacer{
		Stages: []Stage{
			{Duration: 2 * time.Second, Target: 10},
			{Duration: 2 * time.Second, Target: 20},
		},
		Duration: 4 * time.Second,
	}
	// After 1s at 10 QPS, expect ~10 hits scheduled.
	if got := p.expectedHits(time.Second); got < 9 || got > 11 {
		t.Fatalf("expectedHits(1s)=%v, want ~10", got)
	}
	// After 3s: 2s@10 + 1s@20 = 20+20 = 40
	if got := p.expectedHits(3 * time.Second); got < 38 || got > 42 {
		t.Fatalf("expectedHits(3s)=%v, want ~40", got)
	}
	// Slot 0 fires at t=0, matching ConstantPacer's slot-0 schedule.
	if wait, stop := p.Pace(0, 0); stop || wait != 0 {
		t.Fatalf("first tick: wait=%v stop=%v, want 0/false", wait, stop)
	}
	// The next request waits until the integrated rate reaches 1 (~100ms at 10 QPS).
	wait, stop := p.Pace(0, 1)
	if stop || wait <= 0 {
		t.Fatalf("second tick: wait=%v stop=%v, want positive wait", wait, stop)
	}
	if wait > 150*time.Millisecond {
		t.Fatalf("second tick wait=%v, want ~100ms at 10 QPS", wait)
	}
}

// TestConstantPacer_TinyFreqStops pins the NaN guard: a rate small enough to
// overflow the per-slot interval to +Inf makes target = 0×Inf = NaN at slot 0,
// which compares false against every bound and would otherwise flow into an
// implementation-defined time.Duration conversion. The pacer must stop the
// window cleanly instead, as the pre-slot-0 (sent+1) schedule did.
func TestConstantPacer_TinyFreqStops(t *testing.T) {
	t.Parallel()

	p := ConstantPacer{Freq: 1e-300, Duration: time.Second}
	if _, stop := p.Pace(0, 0); !stop {
		t.Fatal("tiny freq slot 0: stop=false, want true")
	}
	if _, stop := p.Pace(0, 1); !stop {
		t.Fatal("tiny freq slot 1: stop=false, want true")
	}
}

// TestLinearStagePacer_SlotZero mirrors the StepStagePacer slot-0 pins:
// request 0 fires at t=0 regardless of the ramp's starting rate, and request 1
// waits for the integrated rate curve to reach 1.
func TestLinearStagePacer_SlotZero(t *testing.T) {
	t.Parallel()

	p := LinearStagePacer{
		Stages: []Stage{
			{Duration: 5 * time.Second, Target: 10},
			{Duration: 5 * time.Second, Target: 50},
		},
		Duration: 10 * time.Second,
	}
	if wait, stop := p.Pace(0, 0); stop || wait != 0 {
		t.Fatalf("first tick: wait=%v stop=%v, want 0/false", wait, stop)
	}
	// The ramp starts at 10 QPS, so hit 1 arrives near 100ms; the binary
	// search resolves to ~1ms granularity.
	wait, stop := p.Pace(0, 1)
	if stop || wait <= 0 {
		t.Fatalf("second tick: wait=%v stop=%v, want positive wait", wait, stop)
	}
	if wait > 150*time.Millisecond {
		t.Fatalf("second tick wait=%v, want ~100ms at a 10 QPS ramp start", wait)
	}
}

func TestLinearStagePacer_interpolates(t *testing.T) {
	t.Parallel()

	p := LinearStagePacer{
		Stages: []Stage{
			{Duration: 5 * time.Second, Target: 10},
			{Duration: 5 * time.Second, Target: 50},
		},
		Duration: 10 * time.Second,
	}
	// Mid-ramp at 2.5s into first stage: rate should be between 10 and 50.
	mid := p.expectedHits(2500 * time.Millisecond)
	early := p.expectedHits(time.Second)
	if mid <= early {
		t.Fatalf("mid=%v early=%v, want mid > early", mid, early)
	}
	if _, stop := p.Pace(10*time.Second, 1000); !stop {
		t.Fatal("past window: stop=false, want true")
	}
}
