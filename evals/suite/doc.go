// Package suite defines the internal suite primitives that back the
// public authoring surface in [go.alis.build/evals].
//
// Three suite kinds share a common shape (name, environments, hooks,
// erased cases) but differ in their case interface. A fourth kind observes
// infrastructure without generating load:
//
//   - [TestSuite]  — cases implement [TestCase] and return an
//     [execution.CaseResult] with `Checks`.
//   - [EvalSuite]  — cases implement [EvalCase]; results carry `Metrics`.
//   - [LoadSuite]  — cases implement [LoadCase] and receive a resolved
//     `mode` + [loadgen.Profile]; results carry a `Summary` plus
//     SLO checks.
//   - [InfraObserveSuite] — cases implement [InfraObserveCase]; results
//     carry Cloud Run and Spanner snapshots over a settled lookback.
//     Cases run concurrently (read-only Monitoring fetches).
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
//   - [WithContext] / [WithEvalContext]         — decorate the outgoing
//     context (stamp caller identity, auth headers, tracing, etc.)
//   - [WithStopOnFailure] / [WithEvalStopOnFailure] — halt-on-first-fail
//
// Load suites use their own set:
//
//   - [WithLoadEnvironment]                     — declare env dependencies
//   - [WithLoadSetup] / [WithLoadTeardown]      — before/after-cases hooks
//   - [WithLoadContext]                         — per-suite context decoration
//   - [WithLoadStopOnFailure]                   — skip dependent later cases
//   - [WithLoadProfileOverride](mode, profile)  — per-mode profile override
//   - [WithCloudRunTargets] / [WithSpannerTargets] — declare infra targets
//   - [WithLookback] — default lookback for infra-observe cases
//
// # Errors
//
// Constructor and registration failures return typed errors from
// [errors.go] (for example [ErrInvalidSuiteName] and [ErrDuplicateCase]) that
// implement [errors.EvalError] and map to
// gRPC codes for RPC validation.
package suite
