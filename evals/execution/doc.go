// Package execution defines the in-process result types the runner emits
// and the mapper serialises onto `evalspb.Run`.
//
// These types are deliberately proto-free: they are the boundary between
// the case-facing world (leaves recorded on [T], SLO checks on load
// cases) and the wire-facing world in [mapper]. Two properties matter:
//
//   - Case adapters (in [suite]) and reporters can consume them without
//     importing generated proto packages.
//   - The runner can rearrange fields (skipped, panicked, setup-error
//     cases) without editing proto structs.
//
// # Shapes
//
//   - [CaseResult]      — one integration or eval case. Carries Checks
//     for tests, Metrics for evals, both are optional so the same struct
//     serves both kinds. [CaseResult.JudgeCallCount] on eval cases
//     records LLM-as-judge metric result entries for this case.
//   - [SuiteResult]     — a list of CaseResults with wall-clock times.
//     [SuiteResult.Judge] and [SuiteResult.JudgeCallCount] carry
//     LLM-as-judge provenance and the per-suite call count roll-up; the
//     mapper conditionally emits these onto
//     [alis.evals.v1.AgentEvalResults.JudgeInfo].
//   - [JudgeInfo]       — provenance struct ([Model], [ModelVersion])
//     embedded in SuiteResult. Zero value signals "no judge context"
//     and suppresses the wire sidecar.
//   - [LoadCaseResult]  — a load case. Carries a [LoadCaseSummary] with
//     aggregate metrics plus a list of [SloCheckResult]s.
//   - [LoadSuiteResult] — a list of LoadCaseResults.
//
// Checks vs SloCheckResults: a Check is a boolean assertion. A
// SloCheckResult is a numeric threshold comparison — it always carries
// the observed value, the configured limit, and the unit so consumers
// can see headroom on passed checks.
package execution
