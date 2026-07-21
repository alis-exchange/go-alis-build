package runner

import (
	"context"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/suite"
)

// testRunsEnvironmentSetupFailed materializes setup-error results for every
// integration-test case when shared environment setup fails.
func (r *Runner) testRunsEnvironmentSetupFailed(
	ctx context.Context,
	runs []suite.TestSuiteRun,
	err error,
	progress func(completed, total int),
	onSuiteComplete TestSuiteCompleteHook,
) []execution.SuiteResult {
	total := suite.TotalTestCases(runs)
	completed := 0
	notify := func() {
		if progress != nil {
			progress(completed, total)
		} else if r != nil && r.progress != nil {
			r.progress(completed, total)
		}
	}

	out := make([]execution.SuiteResult, 0, len(runs))
	for _, run := range runs {
		suiteStart := time.Now()
		cases := make([]execution.CaseResult, 0, len(run.Cases))
		for _, c := range run.Cases {
			cases = append(cases, *result.SetupErrorResult(c.Name(), err))
			completed++
			notify()
		}
		out = append(out, execution.SuiteResult{
			SuiteName: suiteNameFromRun(run),
			Cases:     cases,
			StartTime: suiteStart,
			EndTime:   time.Now(),
		})
		callTestSuiteComplete(ctx, onSuiteComplete, out[len(out)-1])
	}
	return out
}

// evalRunsEnvironmentSetupFailed is [testRunsEnvironmentSetupFailed] for agent-eval runs.
func (r *Runner) evalRunsEnvironmentSetupFailed(
	ctx context.Context,
	runs []suite.EvalSuiteRun,
	err error,
	progress func(completed, total int),
	onSuiteComplete EvalSuiteCompleteHook,
) []execution.SuiteResult {
	total := suite.TotalEvalCases(runs)
	completed := 0
	notify := func() {
		if progress != nil {
			progress(completed, total)
		} else if r != nil && r.progress != nil {
			r.progress(completed, total)
		}
	}

	out := make([]execution.SuiteResult, 0, len(runs))
	for _, run := range runs {
		suiteStart := time.Now()
		cases := make([]execution.CaseResult, 0, len(run.Cases))
		for _, c := range run.Cases {
			cases = append(cases, *result.EvalSetupErrorResult(c.Name(), err))
			completed++
			notify()
		}
		out = append(out, execution.SuiteResult{
			SuiteName: suiteNameFromEvalRun(run),
			Cases:     cases,
			StartTime: suiteStart,
			EndTime:   time.Now(),
		})
		callEvalSuiteComplete(ctx, onSuiteComplete, out[len(out)-1])
	}
	return out
}

// loadRunsEnvironmentSetupFailed is [testRunsEnvironmentSetupFailed] for load-test runs.
func (r *Runner) loadRunsEnvironmentSetupFailed(
	ctx context.Context,
	runs []suite.LoadSuiteRun,
	err error,
	progress func(completed, total int),
	onSuiteComplete LoadSuiteCompleteHook,
) []execution.LoadSuiteResult {
	total := suite.TotalLoadCases(runs)
	completed := 0
	notify := func() {
		if progress != nil {
			progress(completed, total)
		} else if r != nil && r.progress != nil {
			r.progress(completed, total)
		}
	}

	out := make([]execution.LoadSuiteResult, 0, len(runs))
	for _, run := range runs {
		suiteStart := time.Now()
		cases := make([]execution.LoadCaseResult, 0, len(run.Cases))
		for _, c := range run.Cases {
			cases = append(cases, execution.LoadCaseResult{
				Name:   c.Name(),
				Status: evalspb.Status_FAILED,
				Checks: []execution.SloCheckResult{{
					ID:      result.SetupErrorCheckName,
					Status:  evalspb.Status_FAILED,
					Message: err.Error(),
				}},
			})
			completed++
			notify()
		}
		out = append(out, execution.LoadSuiteResult{
			SuiteName: suiteNameFromLoadRun(run),
			Cases:     cases,
			StartTime: suiteStart,
			EndTime:   time.Now(),
		})
		callLoadSuiteComplete(ctx, onSuiteComplete, out[len(out)-1])
	}
	return out
}
