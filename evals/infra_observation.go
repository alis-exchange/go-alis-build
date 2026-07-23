package evals

import (
	"context"

	evalspb "go.alis.build/common/alis/evals/v1"
)

// InfraObservationCaseFunc runs one infra-observation case against a result builder.
type InfraObservationCaseFunc func(context.Context, *InfraObservationResult)

// InfraObservationResult collects protobuf-native observation case data.
type InfraObservationResult struct{}

// InfraObservationSuite defines and runs named infrastructure-observation cases.
type InfraObservationSuite struct {
	core *suiteCore
}

// NewInfraObservationSuite constructs an observation suite with a stable short name.
func NewInfraObservationSuite(name string) *InfraObservationSuite {
	return &InfraObservationSuite{core: newSuiteCore(name, branchInfraObservation)}
}

// AddCase registers a case and returns the same suite for chaining.
func (s *InfraObservationSuite) AddCase(name string, fn InfraObservationCaseFunc) *InfraObservationSuite {
	s.core.addCase(name, fn)
	return s
}

// Run executes all registered cases synchronously and materializes a Run.
func (s *InfraObservationSuite) Run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, false)
}

// RunAndPublish executes the suite and publishes the materialized Run.
func (s *InfraObservationSuite) RunAndPublish(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, true)
}
