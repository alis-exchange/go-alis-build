// Package mapper converts the runner's in-process [execution] result
// types into the `evalspb.Run` wire type that reporters and consumers
// see.
//
// Mapping is intentionally the only place proto shapes appear outside
// the generated proto packages themselves. Runner, suite, and case
// authors deal exclusively in [execution] types; when we change wire
// shape (add a field to `LoadTestResults.Summary`, rename an enum) the
// blast radius stays inside this package.
//
// # Entry points
//
//   - [IntegrationRun] — `SuiteResult` → `evalspb.Run{IntegrationTest: ...}`
//   - [AgentEvalRun]   — `SuiteResult` → `evalspb.Run{AgentEval: ...}`
//   - [LoadRun]        — `LoadSuiteResult` → `evalspb.Run{LoadTest: ...}`
//
// Each stamps common fields (name, type, status via
// [runner.RollupSuiteStatus] / [runner.RollupLoadSuiteStatus], start/end
// times, operation name, batch id, `ALIS_OS_PROJECT`) so the reporter
// receives a fully-formed Run ready to persist.
//
// # Judge sidecar emission
//
// [AgentEvalRun] conditionally emits
// [alis.evals.v1.AgentEvalResults.JudgeInfo]: it is non-nil only when
// the source [execution.SuiteResult.Judge] carries provenance or
// [execution.SuiteResult.JudgeCallCount] is non-zero. This replaces the
// unconditional empty `Judge{}` emission in `evals` v0.1.4, which
// was indistinguishable on the wire from an unpopulated judge run.
// Downstream JSON consumers must treat absent/null as "no judge run"
// and populated as "at least one judge-classified metric produced a
// result".
package mapper
