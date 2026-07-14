package evals

import (
	"strings"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/loadgen"
)

func TestSLOLatencyP99(t *testing.T) {
	t.Parallel()

	slo := SLOLatencyP99(500 * time.Millisecond)
	// Passing case
	m := &loadgen.Metrics{Latency: loadgen.LatencySummary{P99Ms: 320}}
	checks := evaluateSLOs([]SLO{slo}, m)
	if len(checks) != 1 {
		t.Fatalf("len(checks)=%d, want 1", len(checks))
	}
	c := checks[0]
	if c.ID != "latency.p99_ms" || c.Unit != "ms" || c.Limit != 500 || c.Observed != 320 {
		t.Fatalf("check=%+v", c)
	}
	if c.Status != evalspb.Status_PASSED {
		t.Fatalf("status=%v, want PASSED", c.Status)
	}

	// Failing case: message includes both values in ms.
	m = &loadgen.Metrics{Latency: loadgen.LatencySummary{P99Ms: 612.4}}
	checks = evaluateSLOs([]SLO{slo}, m)
	c = checks[0]
	if c.Status != evalspb.Status_FAILED {
		t.Fatalf("status=%v, want FAILED", c.Status)
	}
	if !strings.Contains(c.Message, "612") || !strings.Contains(c.Message, "500") {
		t.Fatalf("message=%q, want observed & limit", c.Message)
	}
}

func TestSLOLatencyP50AndP95IDs(t *testing.T) {
	t.Parallel()

	m := &loadgen.Metrics{Latency: loadgen.LatencySummary{P50Ms: 10, P95Ms: 100}}
	checks := evaluateSLOs([]SLO{
		SLOLatencyP50(20 * time.Millisecond),
		SLOLatencyP95(50 * time.Millisecond),
	}, m)
	if checks[0].ID != "latency.p50_ms" {
		t.Fatalf("checks[0].ID=%q", checks[0].ID)
	}
	if checks[1].ID != "latency.p95_ms" {
		t.Fatalf("checks[1].ID=%q", checks[1].ID)
	}
	if checks[0].Status != evalspb.Status_PASSED {
		t.Fatalf("p50 should pass at 10ms<=20ms")
	}
	if checks[1].Status != evalspb.Status_FAILED {
		t.Fatalf("p95 should fail at 100ms>50ms")
	}
}

func TestSLOErrorRate(t *testing.T) {
	t.Parallel()

	slo := SLOErrorRate(0.01) // 1%

	// 0/100 = 0% — pass, observed stored as percent.
	m := &loadgen.Metrics{RequestCount: 100, ErrorCount: 0}
	c := evaluateSLOs([]SLO{slo}, m)[0]
	if c.Unit != "%" || c.Limit != 1.0 {
		t.Fatalf("unit/limit stored as percent: %+v", c)
	}
	if c.Status != evalspb.Status_PASSED {
		t.Fatalf("status=%v, want PASSED", c.Status)
	}
	if c.Observed != 0 {
		t.Fatalf("Observed=%v, want 0", c.Observed)
	}

	// 5/100 = 5% — fail.
	m = &loadgen.Metrics{RequestCount: 100, ErrorCount: 5}
	c = evaluateSLOs([]SLO{slo}, m)[0]
	if c.Status != evalspb.Status_FAILED {
		t.Fatalf("status=%v, want FAILED", c.Status)
	}
	if c.Observed != 5.0 {
		t.Fatalf("Observed=%v, want 5.0", c.Observed)
	}

	// Zero requests: 0% error rate, passes.
	m = &loadgen.Metrics{RequestCount: 0}
	c = evaluateSLOs([]SLO{slo}, m)[0]
	if c.Status != evalspb.Status_PASSED || c.Observed != 0 {
		t.Fatalf("zero-request case: %+v", c)
	}
}

func TestSLOMinQPS(t *testing.T) {
	t.Parallel()

	slo := SLOMinQPS(50)
	// Pass: observed >= limit
	c := evaluateSLOs([]SLO{slo}, &loadgen.Metrics{ActualQPS: 60})[0]
	if c.ID != "actual_qps" || c.Unit != "rps" || c.Limit != 50 {
		t.Fatalf("check=%+v", c)
	}
	if c.Status != evalspb.Status_PASSED {
		t.Fatalf("status=%v, want PASSED (60 >= 50)", c.Status)
	}
	// Fail: observed < limit
	c = evaluateSLOs([]SLO{slo}, &loadgen.Metrics{ActualQPS: 20})[0]
	if c.Status != evalspb.Status_FAILED {
		t.Fatalf("status=%v, want FAILED (20 < 50)", c.Status)
	}
	if !strings.Contains(c.Message, "below") {
		t.Fatalf("message=%q, want floor language", c.Message)
	}
}

func TestEvaluateSLOs_EmptyOrNil(t *testing.T) {
	t.Parallel()

	if evaluateSLOs(nil, &loadgen.Metrics{}) != nil {
		t.Fatal("nil slos: expected nil result")
	}
	if evaluateSLOs([]SLO{SLOMinQPS(1)}, nil) != nil {
		t.Fatal("nil metrics: expected nil result")
	}
}

func TestSLOStreamTTFB(t *testing.T) {
	t.Parallel()

	slo := SLOStreamTTFB(100 * time.Millisecond)
	pass := &loadgen.Metrics{
		Stream: &loadgen.StreamSummary{StreamCount: 5, TTFB: loadgen.LatencySummary{P99Ms: 80}},
	}
	c := evaluateSLOs([]SLO{slo}, pass)[0]
	if c.ID != "stream.ttfb_p99_ms" || c.Status != evalspb.Status_PASSED {
		t.Fatalf("pass case: %+v", c)
	}

	fail := &loadgen.Metrics{
		Stream: &loadgen.StreamSummary{StreamCount: 5, TTFB: loadgen.LatencySummary{P99Ms: 150}},
	}
	c = evaluateSLOs([]SLO{slo}, fail)[0]
	if c.Status != evalspb.Status_FAILED {
		t.Fatalf("fail case: %+v", c)
	}

	noStream := evaluateSLOs([]SLO{slo}, &loadgen.Metrics{RequestCount: 10})[0]
	if noStream.Status != evalspb.Status_FAILED {
		t.Fatalf("no stream samples: %+v", noStream)
	}
}

func TestSLOMessagesPerSec(t *testing.T) {
	t.Parallel()

	slo := SLOMessagesPerSec(10)
	pass := &loadgen.Metrics{
		Duration: time.Second,
		Stream:   &loadgen.StreamSummary{MessagesSentTotal: 20},
	}
	c := evaluateSLOs([]SLO{slo}, pass)[0]
	if c.ID != "stream.messages_per_sec" || c.Unit != "msg/s" {
		t.Fatalf("check=%+v", c)
	}
	if c.Status != evalspb.Status_PASSED || c.Observed != 20 {
		t.Fatalf("pass case: %+v", c)
	}

	fail := &loadgen.Metrics{
		Duration: time.Second,
		Stream:   &loadgen.StreamSummary{MessagesSentTotal: 5},
	}
	c = evaluateSLOs([]SLO{slo}, fail)[0]
	if c.Status != evalspb.Status_FAILED {
		t.Fatalf("fail case: %+v", c)
	}
}
