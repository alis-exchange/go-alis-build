package loadgen

import (
	"testing"
	"time"
)

func BenchmarkConstantPacer_Pace(b *testing.B) {
	p := ConstantPacer{Freq: 1000, Duration: time.Minute}
	elapsed := 500 * time.Millisecond
	var sent uint64 = 250
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = p.Pace(elapsed, sent)
	}
}

func BenchmarkStepStagePacer_Pace(b *testing.B) {
	p := StepStagePacer{
		Stages: []Stage{
			{Duration: 30 * time.Second, Target: 100},
			{Duration: 30 * time.Second, Target: 500},
		},
		Duration: time.Minute,
	}
	elapsed := 45 * time.Second
	var sent uint64 = 5000
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = p.Pace(elapsed, sent)
	}
}

func BenchmarkLinearStagePacer_Pace(b *testing.B) {
	p := LinearStagePacer{
		Stages: []Stage{
			{Duration: 30 * time.Second, Target: 100},
			{Duration: 30 * time.Second, Target: 500},
		},
		Duration: time.Minute,
	}
	elapsed := 45 * time.Second
	var sent uint64 = 5000
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = p.Pace(elapsed, sent)
	}
}
