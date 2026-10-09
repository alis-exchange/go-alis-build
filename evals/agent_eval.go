package evals

import (
	"context"
	"errors"

	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/validation"
	"google.golang.org/protobuf/proto"
)

// AgentEvalCaseFunc runs one agent-eval case against a result builder.
type AgentEvalCaseFunc func(context.Context, *AgentEvalResult)

var (
	errAgentSessionAlreadySet = errors.New("evals: agent session id already set")
	errNilAgentMetric         = errors.New("evals: nil agent metric")
	errNilAgentJudgeInfo      = errors.New("evals: nil agent judge info")
	errAgentJudgeAlreadySet   = errors.New("evals: agent judge info already set")
)

// AgentEvalResult collects protobuf-native agent-eval case data.
type AgentEvalResult struct {
	validator  *validation.Validator
	sessionID  string
	sessionSet bool
	metrics    []*evalspb.AgentEvalResults_Case_Metric
	judge      *evalspb.AgentEvalResults_JudgeInfo
	judgeSet   bool
	failures   []error
	// notEvaluated is set by SetNotEvaluated and forces NOT_EVALUATED unless
	// the case failed.
	notEvaluated bool
	// notEvaluatedReasons holds one validation message per SetNotEvaluated call.
	notEvaluatedReasons []string
}

func newAgentEvalResult() *AgentEvalResult {
	return &AgentEvalResult{validator: validation.NewValidator()}
}

// Validator returns the case-local validator used for general validation rules.
func (r *AgentEvalResult) Validator() *validation.Validator {
	if r.validator == nil {
		r.validator = validation.NewValidator()
	}
	return r.validator
}

// Fail records a case failure while preserving any data already added.
func (r *AgentEvalResult) Fail(err error) {
	if err == nil {
		return
	}
	r.failures = append(r.failures, err)
}

// defaultNotEvaluatedReason is the validation message SetNotEvaluated records
// when the caller gives no reason.
const defaultNotEvaluatedReason = "evals: case not evaluated"

// SetNotEvaluated marks the case NOT_EVALUATED while keeping every session
// id, metric, judge info and validation already recorded or recorded later.
// A failure still wins: a FAILED metric, a failed validator rule or a call
// to Fail makes the case FAILED.
//
// Each call records one "_evals.case" validation with status NOT_EVALUATED
// and reason as its message, or "evals: case not evaluated" when reason is
// empty. Calling it again keeps the case NOT_EVALUATED and adds one more
// validation. The run rolls up as usual: a NOT_EVALUATED case makes the run
// NOT_EVALUATED unless another case FAILED.
func (r *AgentEvalResult) SetNotEvaluated(reason string) {
	if reason == "" {
		reason = defaultNotEvaluatedReason
	}
	r.notEvaluated = true
	r.notEvaluatedReasons = append(r.notEvaluatedReasons, reason)
}

// SetSessionID records the ADK session identifier for this case.
func (r *AgentEvalResult) SetSessionID(id string) {
	if r.sessionSet {
		r.Fail(errAgentSessionAlreadySet)
		return
	}
	r.sessionID = id
	r.sessionSet = true
}

// AddMetric appends a protobuf-native metric result.
func (r *AgentEvalResult) AddMetric(m *evalspb.AgentEvalResults_Case_Metric) {
	if m == nil {
		r.Fail(errNilAgentMetric)
		return
	}
	r.metrics = append(r.metrics, proto.Clone(m).(*evalspb.AgentEvalResults_Case_Metric))
}

// SetJudgeInfo declares case-level judge provenance and call counts.
//
// Successful case declarations are aggregated into the run-level JudgeInfo:
// model and version must agree across cases, while call and error counts are
// summed. A conflicting declaration fails that case with an "_evals.judge"
// validation and its counts are excluded from the aggregate.
func (r *AgentEvalResult) SetJudgeInfo(j *evalspb.AgentEvalResults_JudgeInfo) {
	if j == nil {
		r.Fail(errNilAgentJudgeInfo)
		return
	}
	if r.judgeSet {
		r.Fail(errAgentJudgeAlreadySet)
		return
	}
	r.judge = proto.Clone(j).(*evalspb.AgentEvalResults_JudgeInfo)
	r.judgeSet = true
}

// AgentEvalSuite defines and runs named agent-evaluation cases.
type AgentEvalSuite struct {
	core *suiteCore
}

// NewAgentEvalSuite constructs an agent-eval suite with a stable short name.
func NewAgentEvalSuite(name string) *AgentEvalSuite {
	return &AgentEvalSuite{core: newSuiteCore(name, branchAgentEval)}
}

// AddCase registers a case and returns the same suite for chaining.
func (s *AgentEvalSuite) AddCase(name string, fn AgentEvalCaseFunc) *AgentEvalSuite {
	s.core.addCase(name, fn)
	return s
}

// Run executes all registered cases synchronously and materializes a Run.
func (s *AgentEvalSuite) Run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, false)
}

// RunAndPublish executes the suite and publishes the materialized Run.
func (s *AgentEvalSuite) RunAndPublish(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, true)
}
