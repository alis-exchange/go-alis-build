// Package execution defines the in-process result types the runner emits
// and the mapper serialises onto `evalspb.Run`.
//
// These types are the in-process result vocabulary between case execution
// and wire mapping. They are not proto-free: the package imports
// [evalspb.Status], load [evalspb.RunLoadTestRequest_Mode], and passes
// Cloud Run / Spanner snapshot protos through on load and infra-observe
// cases. See Decision 0001 in the repository Conductor context for the
// full classification.
//
// What this layer still isolates:
//
//   - Wall-clock suite timing before timestamp conversion
//   - Case [time.Duration] before durationpb mapping
//   - Judge provenance roll-ups ([JudgeInfo], [JudgeCallCount])
//   - Synthetic skipped/failure cases assembled in [go.alis.build/evals/internal/result]
//   - Registration-only metadata not yet on the wire
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
//     aggregate metrics plus a list of [SloCheckResult]s. [LoadCaseResult.CloudRun]
//     and [LoadCaseResult.Spanner] hold diagnostic infra snapshots when targets
//     are declared on the suite.
//   - [LoadSuiteResult] — a list of LoadCaseResults.
//   - [InfraObserveCaseResult] — one standalone infra observation case with
//     resolved lookback, window bounds, and Cloud Run / Spanner snapshots.
//   - [InfraObserveSuiteResult] — a list of InfraObserveCaseResults.
//
// Checks vs SloCheckResults: a Check is a boolean assertion. A
// SloCheckResult is a numeric threshold comparison — it always carries
// the observed value, the configured limit, and the unit so consumers
// can see headroom on passed checks.
package execution
