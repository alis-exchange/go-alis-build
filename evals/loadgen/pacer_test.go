package loadgen

import (
	"testing"
	"time"
)

func TestConstantPacer_Pace(t *testing.T) {
	t.Parallel()

	p := ConstantPacer{Freq: 100, Duration: time.Second}
	// At elapsed 0, sent 0: next request scheduled at 10ms.
	if wait, stop := p.Pace(0, 0); stop || wait != 10*time.Millisecond {
		t.Fatalf("start: wait=%v stop=%v, want 10ms/false", wait, stop)
	}
	// Behind schedule: send immediately.
	if wait, stop := p.Pace(50*time.Millisecond, 0); stop || wait != 0 {
		t.Fatalf("behind: wait=%v stop=%v, want 0/false", wait, stop)
	}
	// On schedule.
	if wait, stop := p.Pace(10*time.Millisecond, 1); stop || wait != 10*time.Millisecond {
		t.Fatalf("on schedule: wait=%v stop=%v, want 10ms/false", wait, stop)
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
	wait, stop := p.Pace(0, 0)
	if stop || wait <= 0 {
		t.Fatalf("first tick: wait=%v stop=%v, want positive wait", wait, stop)
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
