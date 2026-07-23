package evals

import (
	"context"

	evalspb "go.alis.build/common/alis/evals/v1"
)

// AgentEvalCaseFunc runs one agent-eval case against a result builder.
type AgentEvalCaseFunc func(context.Context, *AgentEvalResult)

// AgentEvalResult collects protobuf-native agent-eval case data.
type AgentEvalResult struct{}

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
