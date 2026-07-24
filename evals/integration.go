package evals

import (
	"context"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/validation"
)

// IntegrationCaseFunc runs one integration case against a fresh validator.
type IntegrationCaseFunc func(context.Context, *validation.Validator)

// IntegrationSuite defines and runs named integration-test cases.
type IntegrationSuite struct {
	core *suiteCore
}

// NewIntegrationSuite constructs an integration suite with a stable short name.
func NewIntegrationSuite(name string) *IntegrationSuite {
	return &IntegrationSuite{core: newSuiteCore(name, branchIntegration)}
}

// AddCase registers a case and returns the same suite for chaining.
func (s *IntegrationSuite) AddCase(name string, fn IntegrationCaseFunc) *IntegrationSuite {
	s.core.addCase(name, fn)
	return s
}

// Run executes all registered cases synchronously and materializes a Run.
func (s *IntegrationSuite) Run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, false)
}

// RunAndPublish executes the suite and publishes the materialized Run.
func (s *IntegrationSuite) RunAndPublish(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.core.run(ctx, opts, true)
}
