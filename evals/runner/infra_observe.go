package runner

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/loadinfra"
	"go.alis.build/evals/suite"
)

// InfraObserveRunParams carries request-level options for infra observation runs.
type InfraObserveRunParams struct {
	// RequestLookback is the highest-precedence lookback override when set.
	RequestLookback *time.Duration
}

// RunInfraObserveSuites executes infra observation suite runs. Cases within a
// suite run concurrently; suites run sequentially. When onSuiteComplete is
// non-nil, it is invoked once per suite after that suite's result is
// materialized; hook errors are ignored (best-effort).
func (r *Runner) RunInfraObserveSuites(
	ctx context.Context,
	runs []suite.InfraObserveSuiteRun,
	params InfraObserveRunParams,
	progress func(completed, total int),
	onSuiteComplete InfraObserveSuiteCompleteHook,
) ([]execution.InfraObserveSuiteResult, error) {
	if r == nil {
		return nil, ErrNilRunner{}
	}
	if len(runs) == 0 {
		return nil, nil
	}

	envTeardown, err := setupEnvironments(r.baseCtx(ctx), firstInfraObserveEnvRegistry(runs), collectInfraObserveEnvironmentNames(runs))
	if err != nil {
		return r.infraObserveRunsEnvironmentSetupFailed(ctx, runs, err, progress, onSuiteComplete), nil
	}
	defer envTeardown()

	total := suite.TotalInfraObserveCases(runs)
	completed := 0
	var progressMu sync.Mutex
	markComplete := func() {
		progressMu.Lock()
		completed++
		if progress != nil {
			progress(completed, total)
		} else if r.progress != nil {
			r.progress(completed, total)
		}
		progressMu.Unlock()
	}

	out := make([]execution.InfraObserveSuiteResult, 0, len(runs))
	for _, run := range runs {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		suiteStart := time.Now()
		runCtx := r.outgoingContext(ctx, run.Decorate)
		cases := make([]execution.InfraObserveCaseResult, 0, len(run.Cases))
		var cancelErr error

		setupOK := true
		if run.Setup != nil {
			if err := run.Setup(runCtx); err != nil {
				setupOK = false
				for _, c := range run.Cases {
					cases = append(cases, infraObserveFailedResult(c.Name(), err.Error()))
					markComplete()
				}
			}
		}

		if setupOK {
			runCtx, closeInfra, infraErr := attachInfraClient(runCtx, run.CloudRun, run.Spanner, r.metricClientFactory)
			if infraErr != nil {
				for _, c := range run.Cases {
					cases = append(cases, infraObserveFailedResult(c.Name(), infraErr.Error()))
					markComplete()
				}
			} else {
				func() {
					defer closeInfra()
					cfg := suite.InfraObserveCaseConfig{
						SuiteLookback: run.Lookback,
						CloudRun:      run.CloudRun,
						Spanner:       run.Spanner,
					}
					if params.RequestLookback != nil {
						cfg.RequestLookback = *params.RequestLookback
						cfg.HasRequest = true
					}
					caseResults, err := Execute(ctx, run.Cases,
						func(caseCtx context.Context, c suite.InfraObserveCase) execution.InfraObserveCaseResult {
							cr := runInfraObserveCaseWithRecovery(caseCtx, c, cfg)
							if cr == nil {
								failed := infraObserveFailedResult(c.Name(), "nil case result")
								return failed
							}
							return *cr
						},
						ExecuteOptions{
							Decorate:      func(c context.Context) context.Context { return runCtx },
							StopOnFailure: run.StopOnFailure,
							Sequential:    run.StopOnFailure,
							Concurrency:   r.infraObserveConcurrencyLimit(),
							OnCaseComplete: func(_, _ int) {
								markComplete()
							},
						},
						infraObserveCaseExecuteHooks(),
					)
					cases = append(cases, caseResults...)
					cancelErr = err
				}()
			}
			if err := runSuiteTeardown(runCtx, run.Teardown); err != nil {
				applyInfraObserveTeardownFailure(cases, err)
			}
		}

		out = append(out, execution.InfraObserveSuiteResult{
			SuiteName: suiteNameFromInfraObserveRun(run),
			Cases:     cases,
			StartTime: suiteStart,
			EndTime:   time.Now(),
		})
		callInfraObserveSuiteComplete(ctx, onSuiteComplete, out[len(out)-1])
		if cancelErr != nil {
			return out, cancelErr
		}
	}
	return out, nil
}

// infraObserveFailedResult builds a FAILED case with a synthetic Cloud Run snapshot
// so wire consumers always receive at least one target row on setup errors.
func infraObserveFailedResult(name, message string) execution.InfraObserveCaseResult {
	return execution.InfraObserveCaseResult{
		Name:     name,
		Status:   evalspb.Status_FAILED,
		CloudRun: []*evalspb.CloudRunTargetSnapshot{loadinfra.ConfigFailureSnapshot(message)},
	}
}

// runInfraObserveCaseWithRecovery invokes one infra-observe case with panic recovery.
func runInfraObserveCaseWithRecovery(ctx context.Context, c suite.InfraObserveCase, cfg suite.InfraObserveCaseConfig) (out *execution.InfraObserveCaseResult) {
	defer func() {
		if rec := recover(); rec != nil {
			panicErr := ErrCasePanic{Value: rec, Stack: string(debug.Stack())}
			failed := infraObserveFailedResult(c.Name(), panicErr.Error())
			out = &failed
		}
	}()
	return c.Run(ctx, cfg)
}

// suiteNameFromInfraObserveRun is [suiteNameFromRun] for infra-observation suite runs.
func suiteNameFromInfraObserveRun(run suite.InfraObserveSuiteRun) string {
	if run.Name != "" {
		return run.Name
	}
	if len(run.Cases) == 0 {
		return ""
	}
	return suitePrefix(run.Cases[0].Name())
}

// RollupInfraObserveSuiteStatus returns the rolled-up status for an infra observe suite result.
func RollupInfraObserveSuiteStatus(sr execution.InfraObserveSuiteResult) evalspb.Status {
	statuses := make([]evalspb.Status, len(sr.Cases))
	for i, c := range sr.Cases {
		statuses[i] = c.Status
	}
	return result.RollupRunStatus(statuses)
}

// infraObserveRunsEnvironmentSetupFailed materializes FAILED results for every
// case when shared environment setup fails, preserving LRO progress accounting.
func (r *Runner) infraObserveRunsEnvironmentSetupFailed(
	ctx context.Context,
	runs []suite.InfraObserveSuiteRun,
	err error,
	progress func(completed, total int),
	onSuiteComplete InfraObserveSuiteCompleteHook,
) []execution.InfraObserveSuiteResult {
	total := suite.TotalInfraObserveCases(runs)
	completed := 0
	notify := func() {
		if progress != nil {
			progress(completed, total)
		} else if r.progress != nil {
			r.progress(completed, total)
		}
	}
	out := make([]execution.InfraObserveSuiteResult, 0, len(runs))
	for _, run := range runs {
		suiteStart := time.Now()
		cases := make([]execution.InfraObserveCaseResult, 0, len(run.Cases))
		for _, c := range run.Cases {
			cases = append(cases, infraObserveFailedResult(c.Name(), err.Error()))
			completed++
			notify()
		}
		out = append(out, execution.InfraObserveSuiteResult{
			SuiteName: suiteNameFromInfraObserveRun(run),
			Cases:     cases,
			StartTime: suiteStart,
			EndTime:   time.Now(),
		})
		callInfraObserveSuiteComplete(ctx, onSuiteComplete, out[len(out)-1])
	}
	return out
}

// collectInfraObserveEnvironmentNames deduplicates environment names across infra-observe runs.
func collectInfraObserveEnvironmentNames(runs []suite.InfraObserveSuiteRun) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, run := range runs {
		for _, name := range run.Environments {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}
