// Package runner executes filtered suite runs and turns them into
// [execution] result structs the mapper can wire onto proto responses.
//
// The runner owns four concerns that live outside individual cases:
//
//   - Environment activation via [env.Register]ed hooks. Setup runs once
//     per RunXxxSuites call across all selected suites; teardown runs
//     even when the operation is cancelled.
//   - Context decoration — suite-level [suite.TestSuiteRun.Decorate] /
//     [suite.EvalSuiteRun.Decorate] or the runner-wide [WithContext]
//     transforms the outgoing context handed to hooks and case bodies.
//     Suite-level fully overrides runner-level; both nil means the
//     caller's context is used as-is.
//   - Progress callbacks — the RPC layer wires these into LRO metadata
//     so clients can poll `completed_case_count` / `completed_suite_count`.
//   - Suite-completion hooks — optional per-call [TestSuiteCompleteHook],
//     [EvalSuiteCompleteHook], [LoadSuiteCompleteHook], and
//     [InfraObserveSuiteCompleteHook] callbacks fire once per suite after
//     its result is materialized. Mapping to `evalspb.Run` and emission
//     via [report.Reporter] is a caller concern wired through these hooks.
//   - Panic recovery — a panic in one case is turned into a FAILED case
//     result so a single bad case cannot take down the batch.
//
// # Method surface
//
//   - [Runner.RunTestSuites]  — integration tests, sequential per suite.
//   - [Runner.RunEvalSuites]  — agent evaluations, sequential per suite.
//   - [Runner.RunLoadSuites]  — load tests, always sequential (concurrent
//     load windows against different targets would contaminate each
//     other's measurements).
//   - [Runner.RunInfraObserveSuites] — infra observation, cases within a
//     suite run concurrently; one [loadinfra.MetricClient] per suite run.
//
// Load and infra-observe runs attach a shared Monitoring client to context
// when targets are declared. The client is closed after each suite finishes.
//
// # Load profile resolution
//
// [RunLoadSuites] does not know about the mode-defaults table in the
// parent [evals] package; the caller supplies a [LoadProfileResolver]
// that returns the resolved [loadgen.Profile] for a given (suite, mode)
// pair. This keeps the runtime free of policy and lets tests override
// resolution.
//
// # StopOnFailure
//
// Test and eval suites honour a per-suite `StopOnFailure` flag: once a
// case ends non-PASSED, the remaining cases are recorded with status
// NOT_EVALUATED and a "preceding case … failed" reason. Load suites do
// not honour StopOnFailure — see the package [suite] docs for rationale.
//
// # Rollup
//
// [RollupSuiteStatus], [RollupLoadSuiteStatus], and
// [RollupInfraObserveSuiteStatus] compute the top-level status the mapper stamps on each `evalspb.Run`: PASSED only when every
// case ran and passed; FAILED otherwise (including NOT_EVALUATED cases,
// which are treated as failures at the run level).
package runner
