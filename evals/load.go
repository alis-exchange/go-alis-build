package evals

import (
	"context"

	evalspb "go.alis.build/common/alis/evals/v1"
)

// LoadCaseFunc runs one load case against a result builder.
type LoadCaseFunc func(context.Context, *LoadResult)

// LoadResult collects protobuf-native load-test case data.
type LoadResult struct{}

// LoadSuite defines and runs named load-test cases.
type LoadSuite struct {
	core *suiteCore
}

// NewLoadSuite constructs a load suite with a stable short name.
func NewLoadSuite(name string) *LoadSuite {
	return &LoadSuite{core: newSuiteCore(name, branchLoad)}
}

// AddCase registers a case and returns the same suite for chaining.
func (s *LoadSuite) AddCase(name string, fn LoadCaseFunc) *LoadSuite {
	s.core.addCase(name, fn)
	return s
}

// Run executes all registered cases synchronously and materializes a Run.
func (s *LoadSuite) Run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, false)
}

// RunAndPublish executes the suite and publishes the materialized Run.
func (s *LoadSuite) RunAndPublish(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, true)
}
