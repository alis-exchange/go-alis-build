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
//
// Rationale is the judge model's per-rubric justification, populated by
// LLM-as-judge evaluators. Deterministic evaluators leave it empty. On
// the wire it maps to the optional
// [alis.evals.v1.AgentEvalResults.Case.Metric.RubricScore.rationale]
// field; empty strings are elided from the proto by the shared metrics
// wire converter used by both the runner-level mapper and the ADK
// adapter, so readers can distinguish "no rationale provided" from an
// explicit blank.
type RubricScore struct {
	ID        string
	Status    evalspb.Status
	Score     *float64
	Rationale string
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
	// JudgeCallCount is the per-case count of LLM-as-judge metric result
	// entries observed for this case. It is a lower bound on the actual
	// number of judge model invocations: it counts result entries, not
	// per-invocation samples, so it undercuts JudgeModelOptions.NumSamples
	// fan-out (default 5 in adk-python), per-turn loops inside a single
	// metric, and metrics that make multiple internal judge calls (e.g.
	// hallucinations_v1's segmenter + validator). Set by
	// [go.alis.build/evals/adk.Provider.Run] for the ADK path; custom
	// [suite.EvalCase] implementations that invoke judges out-of-band are
	// responsible for populating this field themselves.
	JudgeCallCount int64
}

// JudgeInfo carries per-suite LLM-as-judge provenance. The zero value
// signals "no judge context" — the mapper treats it as a suppression
// hint and omits the wire [alis.evals.v1.AgentEvalResults.JudgeInfo]
// sidecar in that branch. See [go.alis.build/evals/adk.Agent.JudgeModel]
// for how callers declare provenance.
type JudgeInfo struct {
	Model        string
	ModelVersion string
}

// IsZero reports whether the JudgeInfo carries no provenance to emit.
// Callers use this in combination with [SuiteResult.JudgeCallCount] to
// decide whether to emit the wire JudgeInfo sidecar.
func (j JudgeInfo) IsZero() bool {
	return j == JudgeInfo{}
}

// SuiteResult groups case outcomes for one suite execution.
type SuiteResult struct {
	SuiteName string
	Cases     []CaseResult
	StartTime time.Time
	EndTime   time.Time
	// Judge is the caller-declared judge provenance for this suite. Zero
	// value means "not a judge suite" and the mapper will omit the wire
	// JudgeInfo sidecar unless JudgeCallCount is non-zero.
	Judge JudgeInfo
	// JudgeCallCount is the sum of [CaseResult.JudgeCallCount] across all
	// cases in this suite, populated by the runner for convenience so the
	// mapper does not re-walk the case list. It carries the same
	// lower-bound caveat documented on [CaseResult.JudgeCallCount].
	JudgeCallCount int64
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

// LoadStage mirrors a staged profile step in the execution layer.
type LoadStage struct {
	Duration time.Duration
	Target   float64
}

// LoadCaseSummary is the aggregate outcome for one load case. Field
// semantics match evalspb.LoadTestResults.Summary; mode/target_qps/
// concurrency are the resolved values passed to the generator so consumers
// can interpret ActualQPS.
type LoadCaseSummary struct {
	Mode              evalspb.RunLoadTestRequest_Mode
	TargetQPS         float64
	Concurrency       int32
	Duration          time.Duration
	RequestCount      int64
	ErrorCount        int64
	CheckPassedCount  int64
	CheckFailedCount  int64
	DroppedCount      int64
	ActualQPS         float64
	QPSStages         []LoadStage
	ConcurrencyStages []LoadStage
	Latency           LoadLatency
	ErrorsByCode      map[string]int64
	Stream            *LoadStreamSummary
}

// LoadStreamSummary holds aggregate streaming RPC metrics for a load case.
type LoadStreamSummary struct {
	StreamCount       int64
	MessagesSentTotal int64
	TTFB              LoadLatency
	ResponseLatency   LoadLatency
	TotalDuration     LoadLatency
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
	Tags    map[string]string
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
