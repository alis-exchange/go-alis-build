package runner

import (
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/suite"
)

func testCaseExecuteHooks() ExecuteHooks[suite.TestCase, execution.CaseResult] {
	return ExecuteHooks[suite.TestCase, execution.CaseResult]{
		CaseName: func(c suite.TestCase) string { return c.Name() },
		IsPassed: func(cr execution.CaseResult) bool { return cr.Status == evalspb.Status_PASSED },
		SkippedResult: func(name, reason string) execution.CaseResult {
			return *result.SkippedResult(name, reason)
		},
		CancelledResult: func(name string) execution.CaseResult {
			return *result.CancelledCaseResult(name)
		},
	}
}

func evalCaseExecuteHooks() ExecuteHooks[suite.EvalCase, execution.CaseResult] {
	return ExecuteHooks[suite.EvalCase, execution.CaseResult]{
		CaseName: func(c suite.EvalCase) string { return c.Name() },
		IsPassed: func(cr execution.CaseResult) bool { return cr.Status == evalspb.Status_PASSED },
		SkippedResult: func(name, reason string) execution.CaseResult {
			return *result.EvalSkippedResult(name, reason)
		},
		CancelledResult: func(name string) execution.CaseResult {
			return *result.CancelledEvalCaseResult(name)
		},
	}
}

func loadCaseExecuteHooks() ExecuteHooks[suite.LoadCase, execution.LoadCaseResult] {
	return ExecuteHooks[suite.LoadCase, execution.LoadCaseResult]{
		CaseName: func(c suite.LoadCase) string { return c.Name() },
		IsPassed: func(cr execution.LoadCaseResult) bool { return cr.Status == evalspb.Status_PASSED },
		SkippedResult: func(name, reason string) execution.LoadCaseResult {
			return result.SkippedLoadCaseResult(name, reason)
		},
		CancelledResult: func(name string) execution.LoadCaseResult {
			return result.CancelledLoadCaseResult(name)
		},
	}
}

func infraObserveCaseExecuteHooks() ExecuteHooks[suite.InfraObserveCase, execution.InfraObserveCaseResult] {
	return ExecuteHooks[suite.InfraObserveCase, execution.InfraObserveCaseResult]{
		CaseName: func(c suite.InfraObserveCase) string { return c.Name() },
		IsPassed: func(cr execution.InfraObserveCaseResult) bool { return cr.Status == evalspb.Status_PASSED },
		SkippedResult: func(name, reason string) execution.InfraObserveCaseResult {
			return result.SkippedInfraObserveCaseResult(name, reason)
		},
		CancelledResult: func(name string) execution.InfraObserveCaseResult {
			return result.CancelledInfraObserveCaseResult(name)
		},
	}
}
