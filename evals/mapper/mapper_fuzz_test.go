package mapper

import (
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"google.golang.org/protobuf/proto"
)

func FuzzLoadRunProtoRoundtrip(f *testing.F) {
	f.Add(int64(100), int64(5), 10.0, 50.0, 99.0, 25.0, int64(10), int64(200))
	f.Add(int64(0), int64(0), 0.0, 0.0, 0.0, 0.0, int64(0), int64(0))

	f.Fuzz(func(t *testing.T, requestCount, errorCount int64, p50, p95, p99, actualQPS float64, streamCount, messagesTotal int64) {
		if requestCount < 0 || errorCount < 0 || streamCount < 0 || messagesTotal < 0 {
			return
		}
		if errorCount > requestCount {
			return
		}
		for _, v := range []float64{p50, p95, p99, actualQPS} {
			if v < 0 || v > 1e9 {
				return
			}
		}

		start := time.Unix(1_700_000_000, 0)
		summary := execution.LoadCaseSummary{
			Mode:         evalspb.RunLoadTestRequest_MODERATE,
			TargetQPS:    100,
			Concurrency:  25,
			Duration:     time.Minute,
			RequestCount: requestCount,
			ErrorCount:   errorCount,
			ActualQPS:    actualQPS,
			Latency: execution.LoadLatency{
				P50Ms: p50,
				P95Ms: p95,
				P99Ms: p99,
			},
			ErrorsByCode: map[string]int64{"UNAVAILABLE": errorCount},
		}
		if streamCount > 0 {
			summary.Stream = &execution.LoadStreamSummary{
				StreamCount:       streamCount,
				MessagesSentTotal: messagesTotal,
				TTFB:              execution.LoadLatency{P99Ms: p99},
			}
		}

		sr := execution.LoadSuiteResult{
			SuiteName: "load-suite",
			StartTime: start,
			EndTime:   start.Add(time.Minute),
			Cases: []execution.LoadCaseResult{{
				Name:    "load-suite.case",
				Status:  evalspb.Status_PASSED,
				Tags:    map[string]string{"env": "fuzz"},
				Summary: summary,
				Checks: []execution.SloCheckResult{{
					ID:       "latency.p99_ms",
					Status:   evalspb.Status_PASSED,
					Observed: p99,
					Limit:    500,
					Unit:     "ms",
				}},
			}},
		}

		run := LoadRun(sr, "operations/op-1", "run-fuzz", "batch-fuzz")
		data, err := proto.Marshal(run)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var decoded evalspb.Run
		if err := proto.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if decoded.GetType() != evalspb.Run_LOAD_TEST {
			t.Fatalf("type=%v", decoded.GetType())
		}
		c := decoded.GetLoadTest().GetCases()[0]
		got := c.GetSummary()
		if got.GetRequestCount() != requestCount {
			t.Fatalf("RequestCount=%d, want %d", got.GetRequestCount(), requestCount)
		}
		if got.GetErrorCount() != errorCount {
			t.Fatalf("ErrorCount=%d, want %d", got.GetErrorCount(), errorCount)
		}
		if got.GetActualQps() != actualQPS {
			t.Fatalf("ActualQps=%v, want %v", got.GetActualQps(), actualQPS)
		}
		if got.GetLatency().GetP99Ms() != p99 {
			t.Fatalf("P99Ms=%v, want %v", got.GetLatency().GetP99Ms(), p99)
		}
		if streamCount > 0 {
			if got.GetStream().GetStreamCount() != streamCount {
				t.Fatalf("StreamCount=%d, want %d", got.GetStream().GetStreamCount(), streamCount)
			}
			if got.GetStream().GetMessagesSentTotal() != messagesTotal {
				t.Fatalf("MessagesSentTotal=%d, want %d", got.GetStream().GetMessagesSentTotal(), messagesTotal)
			}
		}
		if c.GetTags()["env"] != "fuzz" {
			t.Fatalf("tags=%v", c.GetTags())
		}
	})
}
