package loadgen

import (
	"errors"
	"math"
	"testing"
	"time"
)

func FuzzProfileValidateRandomInputs(f *testing.F) {
	f.Add(100.0, 10, int64(time.Second), int64(0))
	f.Add(0.0, 1, int64(time.Second), int64(-1))

	f.Fuzz(func(t *testing.T, qps float64, concurrency int, durNs int64, warmupNs int64) {
		if math.IsNaN(qps) || math.IsInf(qps, 0) {
			return
		}
		if concurrency < -1000 || concurrency > 10000 {
			return
		}
		dur := time.Duration(durNs)
		warmup := time.Duration(warmupNs)
		if dur < -time.Hour || dur > time.Hour || warmup < -time.Hour || warmup > time.Hour {
			return
		}

		p := Profile{
			QPS:         qps,
			Concurrency: concurrency,
			Duration:    dur,
			Warmup:      warmup,
		}
		_ = p.validate()
	})
}

func FuzzProfileValidateStagedValid(f *testing.F) {
	f.Add(int32(2000), int32(1000), int32(1000), 10.0, 20.0, 5)
	f.Add(int32(5000), int32(2500), int32(2500), 100.0, 200.0, 25)

	f.Fuzz(func(t *testing.T, totalMs, stage1Ms, stage2Ms int32, qps1, qps2 float64, concurrency int) {
		if totalMs <= 0 || stage1Ms <= 0 || stage2Ms <= 0 {
			return
		}
		if int64(stage1Ms)+int64(stage2Ms) != int64(totalMs) {
			return
		}
		if qps1 <= 0 || qps2 <= 0 || qps1 > 1e4 || qps2 > 1e4 {
			return
		}
		if concurrency < 1 || concurrency > 1000 {
			return
		}
		if math.IsNaN(qps1) || math.IsNaN(qps2) {
			return
		}

		total := time.Duration(totalMs) * time.Millisecond
		p := Profile{
			Concurrency: concurrency,
			Duration:    total,
			QPSStages: []Stage{
				{Duration: time.Duration(stage1Ms) * time.Millisecond, Target: qps1},
				{Duration: time.Duration(stage2Ms) * time.Millisecond, Target: qps2},
			},
		}
		if err := p.validate(); err != nil {
			t.Fatalf("valid staged profile rejected: %+v err=%v", p, err)
		}
		wantPeak := qps1
		if qps2 > wantPeak {
			wantPeak = qps2
		}
		if got := p.EffectiveQPS(); got != wantPeak {
			t.Fatalf("EffectiveQPS=%v, want %v", got, wantPeak)
		}
	})
}

func FuzzProfileValidateStagedInvalidSum(f *testing.F) {
	f.Add(int32(2000), int32(900), int32(900), 10.0, 20.0, 5)

	f.Fuzz(func(t *testing.T, totalMs, stage1Ms, stage2Ms int32, qps1, qps2 float64, concurrency int) {
		if totalMs <= 0 || stage1Ms <= 0 || stage2Ms <= 0 {
			return
		}
		if int64(stage1Ms)+int64(stage2Ms) == int64(totalMs) {
			return
		}
		if qps1 <= 0 || qps2 <= 0 || concurrency < 1 {
			return
		}

		p := Profile{
			Concurrency: concurrency,
			Duration:    time.Duration(totalMs) * time.Millisecond,
			QPSStages: []Stage{
				{Duration: time.Duration(stage1Ms) * time.Millisecond, Target: qps1},
				{Duration: time.Duration(stage2Ms) * time.Millisecond, Target: qps2},
			},
		}
		if err := p.validate(); err == nil {
			t.Fatal("expected validation error when stage durations do not sum to total")
		} else if !errors.Is(err, ErrInvalidProfile{}) {
			t.Fatalf("err=%v, want ErrInvalidProfile", err)
		}
	})
}
