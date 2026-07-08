// Package runner executes filtered suite runs and turns them into
// [execution] result structs the mapper can wire onto proto responses.
//
// The runner owns four concerns that live outside individual cases:
//
//   - Environment activation via [env.Register]ed hooks. Setup runs once
//     per RunXxxSuites call across all selected suites; teardown runs
//     even when the operation is cancelled.
//   - Outgoing identity — suite or runner [WithIdentity] is applied via
//     [auth.Outgoing] to the context handed to each case.
//   - Progress callbacks — the RPC layer wires these into LRO metadata
//     so clients can poll `completed_case_count` / `completed_suite_count`.
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
// [RollupSuiteStatus] and [RollupLoadSuiteStatus] compute the top-level
// status the mapper stamps on each `evalspb.Run`: PASSED only when every
// case ran and passed; FAILED otherwise (including NOT_EVALUATED cases,
// which are treated as failures at the run level).
package runner
