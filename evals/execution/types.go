package execution

import (
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
)

// Check is a deterministic assertion leaf (integration checks).
type Check struct {
	ID      string
	Status  evalspb.Status
	Message string
}

// Rubric is a per-dimension judge result within a criterion (in-process judges).
type Rubric struct {
	ID        string
	Status    evalspb.Status
	Rationale string
}

// Criterion is an LLM-as-judge scored evaluation leaf (in-process judges).
type Criterion struct {
	ID        string
	Status    evalspb.Status
	Score     float64
	Threshold float64
	Rationale string
	Rubric    []Rubric
}

// RubricScore is a per-dimension score within an agent eval metric.
type RubricScore struct {
	ID     string
	Status evalspb.Status
	Score  *float64
}

// Metric is one evaluated agent metric (deterministic check or judge score).
type Metric struct {
	ID        string
	Status    evalspb.Status
	Score     *float64
	Threshold float64
	Message   string
	Rubric    []RubricScore
}

// CaseResult is the internal outcome of one case execution.
type CaseResult struct {
	Name      string
	Status    evalspb.Status
	Checks    []Check // integration deterministic checks
	Metrics   []Metric
	SessionID string
	Duration  time.Duration
}

// SuiteResult groups case outcomes for one suite execution.
type SuiteResult struct {
	SuiteName string
	Cases     []CaseResult
	StartTime time.Time
	EndTime   time.Time
}

// SloCheckResult is one SLO evaluation outcome for a load case (a threshold
// comparison against an aggregate metric). Distinct from Check because it
// carries a numeric observed value, limit, and unit.
type SloCheckResult struct {
	ID       string
	Status   evalspb.Status
	Message  string
	Observed float64
	Limit    float64
	Unit     string
}

// LoadCaseSummary is the aggregate outcome for one load case. Field
// semantics match evalspb.LoadTestResults.Summary; mode/target_qps/
// concurrency are the resolved values passed to the generator so consumers
// can interpret ActualQPS.
type LoadCaseSummary struct {
	Mode         evalspb.RunLoadTestRequest_Mode
	TargetQPS    float64
	Concurrency  int32
	Duration     time.Duration
	RequestCount int64
	ErrorCount   int64
	ActualQPS    float64
	Latency      LoadLatency
	ErrorsByCode map[string]int64
}

// LoadLatency mirrors loadgen.LatencySummary in the execution layer so
// callers of the runner do not depend on the loadgen package.
type LoadLatency struct {
	P50Ms  float64
	P95Ms  float64
	P99Ms  float64
	MinMs  float64
	MeanMs float64
	MaxMs  float64
}

// LoadCaseResult is the internal outcome of one load case execution.
type LoadCaseResult struct {
	Name    string
	Status  evalspb.Status
	Summary LoadCaseSummary
	Checks  []SloCheckResult
}

// LoadSuiteResult groups load case outcomes for one suite execution.
type LoadSuiteResult struct {
	SuiteName string
	Cases     []LoadCaseResult
	StartTime time.Time
	EndTime   time.Time
}
