package evals

import (
	"fmt"

	"go.alis.build/evals/registry"
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

// RegisterIntegration publishes an integration-test suite to DefaultRegistry.
// Panics on invalid arguments.
func RegisterIntegration(s *Suite) {
	if s == nil {
		panic("evals.RegisterIntegration: nil suite")
	}
	if s.kind != KindTest {
		panic(fmt.Errorf("evals.RegisterIntegration: suite %q is not KindTest", s.Name()))
	}
	defaultRegistry.RegisterIntegrationSuite(s.test)
}

// RegisterEval publishes an agent-eval suite to DefaultRegistry. Panics on
// invalid arguments.
func RegisterEval(s *Suite) {
	if s == nil {
		panic("evals.RegisterEval: nil suite")
	}
	if s.kind != KindEval {
		panic(fmt.Errorf("evals.RegisterEval: suite %q is not KindEval", s.Name()))
	}
	defaultRegistry.RegisterAgentEvalSuite(s.eval)
}

// RegisterAgent publishes a lazy agent-eval provider (for example one that
// discovers eval sets from a deployed ADK agent) to DefaultRegistry.
func RegisterAgent(p registry.AgentEvalProvider) {
	if p == nil {
		panic("evals.RegisterAgent: nil provider")
	}
	defaultRegistry.RegisterAgentEvalProvider(p)
}

// RegisterLoad publishes a load-test suite to DefaultRegistry. Panics on
// invalid arguments.
func RegisterLoad(s *LoadSuite) {
	if s == nil {
		panic("evals.RegisterLoad: nil suite")
	}
	defaultRegistry.RegisterLoadSuite(s.inner)
}
