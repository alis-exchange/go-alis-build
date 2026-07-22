package evals

import (
	"go.alis.build/evals/registry"
	"go.alis.build/evals/suite"
)

// defaultRegistry is the process-wide registry the deployed TestService
// consumes. Case packages register into it during init via
// RegisterIntegration, RegisterEval, and RegisterAgent (mirrors
// http.DefaultServeMux).
var defaultRegistry = registry.New()

// DefaultRegistry returns the process-wide registry. TestServiceServer is
// constructed with this registry so all cases registered here are reachable
// via the TestService RPCs.
func DefaultRegistry() *registry.Registry {
	return defaultRegistry
}

// Freeze validates and seals DefaultRegistry against further registration.
func Freeze() error {
	return defaultRegistry.Freeze()
}

// RegisterIntegration publishes an integration-test suite to
// DefaultRegistry. Returns [suite.ErrNilSuite] for a nil suite or
// [ErrWrongSuiteKind] when the suite is not KindTest.
func RegisterIntegration(s *Suite) error {
	if s == nil {
		return suite.ErrNilSuite{}
	}
	if s.kind != KindTest {
		return ErrWrongSuiteKind{Suite: s.Name(), Want: KindTest, Got: s.kind}
	}
	return defaultRegistry.RegisterIntegrationSuite(s.test)
}

// RegisterEval publishes an agent-eval suite to DefaultRegistry. Returns
// [suite.ErrNilSuite] for a nil suite or [ErrWrongSuiteKind] when the
// suite is not KindEval.
func RegisterEval(s *Suite) error {
	if s == nil {
		return suite.ErrNilSuite{}
	}
	if s.kind != KindEval {
		return ErrWrongSuiteKind{Suite: s.Name(), Want: KindEval, Got: s.kind}
	}
	return defaultRegistry.RegisterAgentEvalSuite(s.eval)
}

// RegisterAgent publishes a lazy agent-eval provider (for example one
// that discovers eval sets from a deployed ADK agent) to DefaultRegistry.
// Returns [ErrNilProvider] for a nil provider.
func RegisterAgent(p registry.AgentEvalProvider) error {
	if p == nil {
		return ErrNilProvider{}
	}
	return defaultRegistry.RegisterAgentEvalProvider(p)
}

// RegisterLoad publishes a load-test suite to DefaultRegistry. Returns
// [suite.ErrNilSuite] for a nil suite.
func RegisterLoad(s *LoadSuite) error {
	if s == nil {
		return suite.ErrNilSuite{}
	}
	return defaultRegistry.RegisterLoadSuite(s.inner)
}

// RegisterInfraObserve publishes an infra observation suite to DefaultRegistry.
// Returns [suite.ErrNilSuite] for a nil suite.
func RegisterInfraObserve(s *InfraObserveSuite) error {
	if s == nil {
		return suite.ErrNilSuite{}
	}
	if err := s.inner.Validate(); err != nil {
		return err
	}
	return defaultRegistry.RegisterInfraObserveSuite(s.inner)
}
