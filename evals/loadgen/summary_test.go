package loadgen

import (
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestSummary_mapsMetricsProfileAndStages(t *testing.T) {
	t.Parallel()

	profile := Profile{
		QPS:         100,
		Concurrency: 8,
		Duration:    30 * time.Second,
		Warmup:      10 * time.Second,
		QPSStages: []Stage{
			{Duration: 20 * time.Second, Target: 50},
			{Duration: 20 * time.Second, Target: 150},
		},
		ConcurrencyStages: []Stage{
			{Duration: 20 * time.Second, Target: 4},
			{Duration: 20 * time.Second, Target: 8},
		},
	}
	metrics := &Metrics{
		Duration:         30 * time.Second,
		RequestCount:     3000,
		ErrorCount:       3,
		CheckPassedCount: 2990,
		CheckFailedCount: 7,
		DroppedCount:     11,
		ActualQPS:        99.5,
		Latency: LatencySummary{
			P50Ms:  10,
			P95Ms:  25,
			P99Ms:  40,
			MinMs:  2,
			MeanMs: 12,
			MaxMs:  80,
		},
		ErrorsByCode: map[string]int64{
			"UNAVAILABLE":       2,
			"DEADLINE_EXCEEDED": 1,
		},
	}

	got := Summary(evalspb.RunLoadTestRequest_HIGH, profile, metrics)
	want := &evalspb.LoadTestResults_Summary{
		Mode:             evalspb.RunLoadTestRequest_HIGH,
		TargetQps:        150,
		Concurrency:      8,
		Duration:         durationpb.New(30 * time.Second),
		RequestCount:     3000,
		ErrorCount:       3,
		CheckPassedCount: 2990,
		CheckFailedCount: 7,
		DroppedCount:     11,
		ActualQps:        99.5,
		Latency: &evalspb.LatencyPercentiles{
			P50Ms:  10,
			P95Ms:  25,
			P99Ms:  40,
			MinMs:  2,
			MeanMs: 12,
			MaxMs:  80,
		},
		ErrorsByCode: []*evalspb.LoadTestResults_Int64Entry{
			{Key: "DEADLINE_EXCEEDED", Value: 1},
			{Key: "UNAVAILABLE", Value: 2},
		},
		QpsStages: []*evalspb.LoadTestResults_LoadStage{
			{Duration: durationpb.New(20 * time.Second), Target: 50},
			{Duration: durationpb.New(20 * time.Second), Target: 150},
		},
		ConcurrencyStages: []*evalspb.LoadTestResults_LoadStage{
			{Duration: durationpb.New(20 * time.Second), Target: 4},
			{Duration: durationpb.New(20 * time.Second), Target: 8},
		},
	}
	if !proto.Equal(got, want) {
		t.Fatalf("Summary() = %v, want %v", got, want)
	}
}

func TestSummary_mapsStreamSummary(t *testing.T) {
	t.Parallel()

	got := Summary(evalspb.RunLoadTestRequest_MODERATE, Profile{
		QPS:         10,
		Concurrency: 2,
		Duration:    time.Second,
	}, &Metrics{
		Duration: time.Second,
		Stream: &StreamSummary{
			StreamCount:       3,
			MessagesSentTotal: 9,
			TTFB:              LatencySummary{P99Ms: 7},
			ResponseLatency:   LatencySummary{P95Ms: 11},
			TotalDuration:     LatencySummary{MeanMs: 20},
		},
	})
	stream := got.GetStream()
	if stream == nil {
		t.Fatal("stream = nil, want populated")
	}
	if stream.GetStreamCount() != 3 || stream.GetMessagesSentTotal() != 9 {
		t.Fatalf("stream counts = %+v, want 3/9", stream)
	}
	if stream.GetTtfb().GetP99Ms() != 7 {
		t.Fatalf("ttfb.p99 = %v, want 7", stream.GetTtfb().GetP99Ms())
	}
	if stream.GetResponseLatency().GetP95Ms() != 11 {
		t.Fatalf("response_latency.p95 = %v, want 11", stream.GetResponseLatency().GetP95Ms())
	}
	if stream.GetTotalDuration().GetMeanMs() != 20 {
		t.Fatalf("total_duration.mean = %v, want 20", stream.GetTotalDuration().GetMeanMs())
	}
}

func TestSummary_nilAndZeroBehavior(t *testing.T) {
	t.Parallel()

	if got := Summary(evalspb.RunLoadTestRequest_MINIMAL, Profile{}, nil); got != nil {
		t.Fatalf("Summary(nil metrics) = %v, want nil", got)
	}
	got := Summary(evalspb.RunLoadTestRequest_MINIMAL, Profile{}, &Metrics{})
	if got == nil {
		t.Fatal("Summary(zero metrics) = nil, want zero summary")
	}
	if got.GetLatency() == nil {
		t.Fatal("zero summary latency = nil, want zero-valued latency object")
	}
	if got.GetStream() != nil {
		t.Fatalf("zero summary stream = %v, want nil", got.GetStream())
	}
}
