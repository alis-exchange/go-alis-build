package loadgen

import (
	"math"
	"testing"
	"time"
)

func FuzzConstantPacer_Pace(f *testing.F) {
	f.Add(100.0, int64(time.Second), int64(0), uint64(0))
	f.Add(1000.0, int64(500*time.Millisecond), int64(250*time.Millisecond), uint64(100))

	f.Fuzz(func(t *testing.T, freq float64, durNs int64, elapsedNs int64, sent uint64) {
		if math.IsNaN(freq) || math.IsInf(freq, 0) {
			return
		}
		if freq < 0 || freq > 1e6 {
			return
		}
		dur := time.Duration(durNs)
		elapsed := time.Duration(elapsedNs)
		if dur < 0 || elapsed < 0 {
			return
		}

		wait, stop := ConstantPacer{Freq: freq, Duration: dur}.Pace(elapsed, sent)
		if wait < 0 {
			t.Fatalf("negative wait: freq=%v dur=%v elapsed=%v sent=%d wait=%v", freq, dur, elapsed, sent, wait)
		}
		if dur > 0 && elapsed >= dur && !stop {
			t.Fatalf("expected stop when elapsed >= duration")
		}
		if freq <= 0 && !stop {
			t.Fatalf("expected stop when freq <= 0")
		}
	})
}

func FuzzStepStagePacer_expectedHitsMonotonic(f *testing.F) {
	f.Add(int32(1000), int32(1000), 10.0, 20.0, int32(100), int32(500))
	f.Add(int32(500), int32(1500), 5.0, 50.0, int32(0), int32(2000))

	f.Fuzz(func(t *testing.T, seg1Ms, seg2Ms int32, qps1, qps2 float64, elapsed1Ms, elapsed2Ms int32) {
		if seg1Ms <= 0 || seg2Ms <= 0 {
			return
		}
		if qps1 <= 0 || qps2 <= 0 || qps1 > 1e4 || qps2 > 1e4 {
			return
		}
		if math.IsNaN(qps1) || math.IsNaN(qps2) {
			return
		}
		if elapsed1Ms < 0 || elapsed2Ms < 0 {
			return
		}
		if elapsed2Ms < elapsed1Ms {
			elapsed1Ms, elapsed2Ms = elapsed2Ms, elapsed1Ms
		}

		total := time.Duration(seg1Ms+seg2Ms) * time.Millisecond
		p := StepStagePacer{
			Stages: []Stage{
				{Duration: time.Duration(seg1Ms) * time.Millisecond, Target: qps1},
				{Duration: time.Duration(seg2Ms) * time.Millisecond, Target: qps2},
			},
			Duration: total,
		}
		e1 := p.expectedHits(time.Duration(elapsed1Ms) * time.Millisecond)
		e2 := p.expectedHits(time.Duration(elapsed2Ms) * time.Millisecond)
		if e2+1e-9 < e1 {
			t.Fatalf("expectedHits not monotonic: e(%dms)=%v e(%dms)=%v", elapsed1Ms, e1, elapsed2Ms, e2)
		}
		if e1 < 0 || e2 < 0 {
			t.Fatalf("negative expected hits: e1=%v e2=%v", e1, e2)
		}
	})
}

func FuzzLinearStagePacer_expectedHitsMonotonic(f *testing.F) {
	f.Add(int32(1000), int32(1000), 10.0, 50.0, int32(100), int32(500))

	f.Fuzz(func(t *testing.T, seg1Ms, seg2Ms int32, startQPS, endQPS float64, elapsed1Ms, elapsed2Ms int32) {
		if seg1Ms <= 0 || seg2Ms <= 0 {
			return
		}
		if startQPS <= 0 || endQPS <= 0 || startQPS > 1e4 || endQPS > 1e4 {
			return
		}
		if math.IsNaN(startQPS) || math.IsNaN(endQPS) {
			return
		}
		if elapsed1Ms < 0 || elapsed2Ms < 0 {
			return
		}
		if elapsed2Ms < elapsed1Ms {
			elapsed1Ms, elapsed2Ms = elapsed2Ms, elapsed1Ms
		}

		total := time.Duration(seg1Ms+seg2Ms) * time.Millisecond
		p := LinearStagePacer{
			Stages: []Stage{
				{Duration: time.Duration(seg1Ms) * time.Millisecond, Target: startQPS},
				{Duration: time.Duration(seg2Ms) * time.Millisecond, Target: endQPS},
			},
			Duration: total,
		}
		e1 := p.expectedHits(time.Duration(elapsed1Ms) * time.Millisecond)
		e2 := p.expectedHits(time.Duration(elapsed2Ms) * time.Millisecond)
		if e2+1e-9 < e1 {
			t.Fatalf("expectedHits not monotonic: e(%dms)=%v e(%dms)=%v", elapsed1Ms, e1, elapsed2Ms, e2)
		}
	})
}
