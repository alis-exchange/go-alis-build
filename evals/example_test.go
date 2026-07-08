package evals_test

import (
	"context"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals"
)

// fakeItem stands in for a caller's typed protobuf response so the examples
// compile without pulling in a real client library.
type fakeItem struct {
	Name string
	Size int64
}

func (f *fakeItem) GetName() string { return f.Name }
func (f *fakeItem) GetSize() int64  { return f.Size }

// fetchItem is a placeholder for whatever gRPC client method a real suite
// would call. Example functions do not exercise it; they demonstrate the
// shape of the call site.
func fetchItem(ctx context.Context, name string) (*fakeItem, error) {
	return &fakeItem{Name: name, Size: 42}, nil
}

// ExampleCall shows how to wrap a single RPC so its response, error, and
// latency are captured in one shot. The result is then asserted with the
// per-case recorder T. Call itself records nothing; every assertion is
// explicit.
func ExampleCall() {
	s := evals.MustNewIntegrationSuite("example-v1")
	s.MustCase("get-item", func(ctx context.Context, t *evals.T) {
		r := evals.Call(ctx, func(ctx context.Context) (*fakeItem, error) {
			return fetchItem(ctx, "items/root")
		})
		if !t.NoErr("grpc", r.Err) {
			return
		}
		if !t.Max("latency", r.Latency, 500*time.Millisecond) {
			return
		}
		t.Check("has-name", r.Resp.GetName() != "")
	})
	// evals.RegisterIntegration(s) — omitted to keep the example
	// side-effect free.
	_ = s
}

// ExampleT_Score shows the eval-suite scoring pattern. T.Score records the
// observed value, the pass threshold, and any rationale so consumers see
// how much headroom each metric had.
func ExampleT_Score() {
	s := evals.MustNewAgentEvalSuite("example-agent-v1")
	s.MustCase("golden-summary", func(ctx context.Context, t *evals.T) {
		// A real case would call the agent here; the string literals stand
		// in for the produced response and its golden reference.
		got := "the quick brown fox"
		golden := "the quick brown fox jumped"

		// Deterministic scorer bundled with the framework. Feed its output
		// into t.Score so both the score and its threshold land on the wire.
		t.Score("rouge-1", evals.Rouge1F1(got, golden), 0.5, "vs golden reference")
	})
	_ = s
}

// ExampleNewLoadSuite shows the minimum wiring for a load suite: a target
// function and a small set of SLOs. The framework owns pacing, warmup, and
// error accounting.
func ExampleNewLoadSuite() {
	s := evals.MustNewLoadSuite("example-v1-load",
		// Override the MODERATE preset for this suite specifically. Other
		// modes keep the framework defaults.
		evals.WithLoadProfile(evalspb.RunLoadTestRequest_MODERATE, evals.Profile{
			QPS:            100,
			Concurrency:    20,
			Duration:       60 * time.Second,
			Warmup:         10 * time.Second,
			RequestTimeout: 5 * time.Second,
		}),
	)

	s.MustLoadCase("get-item",
		func(ctx context.Context) error {
			_, err := fetchItem(ctx, "items/root")
			return err
		},
		evals.SLOLatencyP99(500*time.Millisecond),
		evals.SLOErrorRate(0.01),
		evals.SLOMinQPS(20),
	)

	// evals.RegisterLoad(s) — omitted here to keep the example
	// side-effect free; register at process init in real code.
	_ = s
}

// ExampleSLOLatencyP99 shows how to declare a tail-latency guardrail
// alongside a load case. Every declared SLO produces one SloCheck per case
// run — passed or failed — so consumers can compute headroom on passing
// checks, not just breaches.
func ExampleSLOLatencyP99() {
	slo := evals.SLOLatencyP99(300 * time.Millisecond)

	s := evals.MustNewLoadSuite("example-v1-load")
	s.MustLoadCase("get-item",
		func(ctx context.Context) error {
			_, err := fetchItem(ctx, "items/root")
			return err
		},
		slo,
	)
	_ = s
}
