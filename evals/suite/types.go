package suite

import (
	"context"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadgen"
)

// TestCase is the erased, runnable unit an integration suite executes.
// Implementations may compose scenarios, assertions, and judges internally;
// the runner only sees Name and Run.
type TestCase interface {
	Name() string
	Run(ctx context.Context) *execution.CaseResult
}

// EvalCase is the erased, runnable unit an agent-eval suite executes.
type EvalCase interface {
	Name() string
	Run(ctx context.Context) *execution.CaseResult
}

// LoadCase is the erased, runnable unit a load suite executes. Unlike
// TestCase / EvalCase the runner supplies a resolved Profile (from the
// requested mode + any suite override), and the case owns invoking the
// generator and evaluating its SLOs against the returned metrics.
type LoadCase interface {
	Name() string
	Run(ctx context.Context, mode evalspb.RunLoadTestRequest_Mode, profile loadgen.Profile) *execution.LoadCaseResult
}

// InfraObserveCase is the erased, runnable unit an infra observation suite
// executes. The runner supplies resolved lookback and infra targets; the case
// fetches Monitoring snapshots over the settled lookback window.
type InfraObserveCase interface {
	Name() string
	// Lookback returns a per-case lookback override when the second value is true.
	Lookback() (time.Duration, bool)
	Run(ctx context.Context, cfg InfraObserveCaseConfig) *execution.InfraObserveCaseResult
}
