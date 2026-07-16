// Package registry holds every suite the TestServiceServer can execute
// and resolves case_ids filters into concrete suite runs.
//
// A [Registry] is populated at startup — case packages call
// [Registry.RegisterIntegrationSuite], [Registry.RegisterAgentEvalSuite],
// [Registry.RegisterLoadSuite], [Registry.RegisterInfraObserveSuite], and
// [Registry.RegisterAgentEvalProvider]
// during package init via the wrappers in the parent [evals] package.
// TestServiceServer is constructed with [evals.DefaultRegistry] so all
// case packages published to that registry are reachable via the RPCs.
//
// # Selection
//
// The RPC's case_ids field is a filter, not an enumeration:
//
//   - empty     → run every registered suite for the requested type
//   - "suite"   → run every case in that suite
//   - "suite.a" → run just that case
//
// Selection is performed by [Registry.SelectTestRuns],
// [Registry.SelectEvalRuns], [Registry.SelectLoadRuns], and
// [Registry.SelectInfraObserveRuns]. Each returns
// suite-scoped run objects that carry only the matching cases; the runner
// iterates these directly.
//
// Validation ([Registry.ValidateSelection]) is called before the LRO is
// created so the client gets an InvalidArgument for unknown case ids
// synchronously, rather than a failed operation.
//
// # Agent-eval providers
//
// In addition to static [suite.EvalSuite]s, agent evaluations can be
// sourced lazily via [AgentEvalProvider]. The [adk] subpackage registers
// one such provider that discovers eval sets from a deployed ADK agent
// at run time. Providers own their own filtering — the registry passes
// the raw case_ids through.
//
// # Errors
//
// [ErrNoSuites], [ErrNoEvalSuites], [ErrNoLoadSuites],
// [ErrNoInfraObserveSuites], [ErrUnknownCase],
// [ErrUnsupportedRunType], and [ErrNotConfigured] all implement
// [errors.EvalError] and map to appropriate gRPC codes.
package registry
