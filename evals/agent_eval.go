package evals

import (
	"context"
	"errors"

	evalspb "go.alis.build/common/alis/evals/v1"
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
