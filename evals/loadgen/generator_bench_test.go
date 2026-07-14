package loadgen

import (
	"context"
	"errors"
	"testing"
	"time"
)

func benchAggregator() (*aggregator, sample) {
	start := time.Now().Add(-time.Hour)
	agg := newAggregator(start, start.Add(2*time.Hour), time.Minute, 0)
	s := sample{
		sentAt:  time.Now(),
		latency: 12 * time.Millisecond,
		result: TargetResult{
			TransportErr: errors.New("UNAVAILABLE"),
			Stream: &StreamSample{
				SendDuration:    5 * time.Millisecond,
				ResponseLatency: 2 * time.Millisecond,
				TotalDuration:   8 * time.Millisecond,
				MessagesSent:    3,
			},
		},
	}
	return agg, s
}

func BenchmarkAggregatorRecord(b *testing.B) {
	agg, s := benchAggregator()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agg.record(s)
	}
}

func BenchmarkAggregatorRecordParallel(b *testing.B) {
	agg, s := benchAggregator()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			agg.record(s)
		}
	})
}

func BenchmarkAggregatorAbortSnapshot(b *testing.B) {
	agg, s := benchAggregator()
	for i := 0; i < 1000; i++ {
		agg.record(s)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agg.abortSnapshot()
	}
}

func BenchmarkAggregatorFinalize(b *testing.B) {
	agg, s := benchAggregator()
	for i := 0; i < 1000; i++ {
		agg.record(s)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agg.finalize()
	}
}

func BenchmarkGeneratorRun(b *testing.B) {
	g := New()
	cases := []struct {
		name string
		qps  float64
		conc int
	}{
		{"QPS100", 100, 10},
		{"QPS1000", 1000, 50},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			p := Profile{
				QPS:         tc.qps,
				Concurrency: tc.conc,
				Duration:    100 * time.Millisecond,
			}
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := g.Run(ctx, p, zeroLatencyTarget)
				if err != nil {
					b.Fatalf("Run: %v", err)
				}
			}
		})
	}
}
