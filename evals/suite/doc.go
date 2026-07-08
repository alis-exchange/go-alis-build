// Package suite defines the internal suite primitives that back the
// public authoring surface in [go.alis.build/evals].
//
// Three suite kinds share a common shape (name, environments, hooks,
// erased cases) but differ in their case interface:
//
//   - [TestSuite]  — cases implement [TestCase] and return an
//     [execution.CaseResult] with `Checks`.
//   - [EvalSuite]  — cases implement [EvalCase]; results carry `Metrics`.
//   - [LoadSuite]  — cases implement [LoadCase] and receive a resolved
//     `mode` + [loadgen.Profile]; results carry a `Summary` plus
//     SLO checks.
//
// All three qualify short case names ("upload") to "{suite}.{case}" at
// registration and enforce uniqueness within a suite. Case names must not
// contain '.' — that character is reserved for the filter grammar.
//
// # Filtering
//
// [ParseFilterPaths] converts the case_ids field on the RPC into
// [FilterPath] values. Each `SelectXxxCases` method returns the cases in
// a suite that match a given filter set:
//
//   - empty filters                  → all cases
//   - filter mentions the suite only ("files-v2") → all cases in that suite
//   - qualified filter ("files-v2.get-root")     → just that case
//
// A suite that isn't mentioned in a non-empty filter set contributes no
// cases (nil), which the registry uses to drop suites from the run
// entirely.
//
// # Lifecycle hooks
//
// Every suite kind accepts optional Setup and Teardown [SuiteHook]s.
// Setup failure fails every case in the suite with a setup-error marker
// and skips teardown; teardown failure is logged but does not affect case
// outcomes. See [runner.Runner] for the execution sequence.
//
// # Options
//
// Test and eval suites share option semantics:
//
//   - [WithEnvironment] / [WithEvalEnvironment] — declare env dependencies
//   - [WithSetup] / [WithEvalSetup]             — before-cases hook
//   - [WithTeardown] / [WithEvalTeardown]       — after-cases hook
//   - [WithIdentity] / [WithEvalIdentity]       — simulate caller identity
//   - [WithStopOnFailure] / [WithEvalStopOnFailure] — halt-on-first-fail
//
// Load suites use their own set:
//
//   - [WithLoadEnvironment]                     — declare env dependencies
//   - [WithLoadSetup] / [WithLoadTeardown]      — before/after-cases hooks
//   - [WithLoadProfileOverride](mode, profile)  — per-mode profile override
//
// Load suites deliberately have no identity option: load tests should
// run under the service's default identity so their measurements match
// production traffic.
//
// # Errors
//
// Constructor and registration failures return typed errors from
// [errors.go] (for example [ErrInvalidSuiteName], [ErrDuplicateCase],
// [ErrUnknownEnvironment]) that implement [errors.EvalError] and map to
// gRPC codes for RPC validation.
package suite
