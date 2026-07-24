package loadgen

import (
	"sort"

	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Summary converts generated load metrics into the wire summary shape.
func Summary(mode evalspb.RunLoadTestRequest_Mode, profile Profile, metrics *Metrics) *evalspb.LoadTestResults_Summary {
	if metrics == nil {
		return nil
	}
	return &evalspb.LoadTestResults_Summary{
		Mode:              mode,
		TargetQps:         profile.EffectiveQPS(),
		Concurrency:       int32(profile.MaxConcurrency()),
		Duration:          durationpb.New(metrics.Duration),
		RequestCount:      metrics.RequestCount,
		ErrorCount:        metrics.ErrorCount,
		CheckPassedCount:  metrics.CheckPassedCount,
		CheckFailedCount:  metrics.CheckFailedCount,
		DroppedCount:      metrics.DroppedCount,
		ActualQps:         metrics.ActualQPS,
		Latency:           latencyProto(metrics.Latency),
		ErrorsByCode:      int64Entries(metrics.ErrorsByCode),
		QpsStages:         stagesProto(profile.QPSStages),
		ConcurrencyStages: stagesProto(profile.ConcurrencyStages),
		Stream:            streamProto(metrics.Stream),
	}
}

func stagesProto(stages []Stage) []*evalspb.LoadTestResults_LoadStage {
	if len(stages) == 0 {
		return nil
	}
	out := make([]*evalspb.LoadTestResults_LoadStage, len(stages))
	for i, s := range stages {
		out[i] = &evalspb.LoadTestResults_LoadStage{
			Duration: durationpb.New(s.Duration),
			Target:   s.Target,
		}
	}
	return out
}

func streamProto(s *StreamSummary) *evalspb.LoadTestResults_StreamSummary {
	if s == nil {
		return nil
	}
	return &evalspb.LoadTestResults_StreamSummary{
		StreamCount:       s.StreamCount,
		MessagesSentTotal: s.MessagesSentTotal,
		Ttfb:              latencyProtoNonZero(s.TTFB),
		ResponseLatency:   latencyProtoNonZero(s.ResponseLatency),
		TotalDuration:     latencyProtoNonZero(s.TotalDuration),
	}
}

func latencyProtoNonZero(l LatencySummary) *evalspb.LatencyPercentiles {
	if l == (LatencySummary{}) {
		return nil
	}
	return latencyProto(l)
}

func latencyProto(l LatencySummary) *evalspb.LatencyPercentiles {
	return &evalspb.LatencyPercentiles{
		P50Ms:  l.P50Ms,
		P95Ms:  l.P95Ms,
		P99Ms:  l.P99Ms,
		MinMs:  l.MinMs,
		MeanMs: l.MeanMs,
		MaxMs:  l.MaxMs,
	}
}

func int64Entries(m map[string]int64) []*evalspb.LoadTestResults_Int64Entry {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*evalspb.LoadTestResults_Int64Entry, 0, len(m))
	for _, k := range keys {
		out = append(out, &evalspb.LoadTestResults_Int64Entry{Key: k, Value: m[k]})
	}
	return out
}
