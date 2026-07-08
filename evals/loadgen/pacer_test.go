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
