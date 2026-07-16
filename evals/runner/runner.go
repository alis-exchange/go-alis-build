package runner

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/suite"
)

// Runner executes registered suites against a target and reports outcomes.
//
// The zero value is not usable; construct one with [New]. A Runner is safe
// to reuse across LROs but is not itself goroutine-safe: give each
// concurrent LRO its own Runner, or serialise access.
//
// Cross-suite parallelism is deliberately absent. Test and eval suites run
// sequentially so the reporter emits Runs in a stable order and setup
// hooks don't race; load suites run sequentially because concurrent load
// windows against different targets would contaminate one another's
// latency samples. Case-level concurrency is a load-suite concern
// expressed via [loadgen.Profile.Concurrency].
type Runner struct {
	// progress is the default LRO progress callback when a Run* method receives nil progress.
	progress func(completed, total int)
	// decorate is the runner-wide ContextDecorator inherited by suites without their own.
	decorate suite.ContextDecorator
	// abortOnSLOFailure enables mid-load cancellation when partial metrics breach an SLO.
	abortOnSLOFailure bool
}

// Option configures a [Runner] at construction time.
type Option func(*Runner)

// New constructs a [Runner] with the given options.
func New(opts ...Option) *Runner {
	r := &Runner{}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// WithProgress installs a progress callback that fires once per completed
// case as (completed, total). Callers of RunTestSuites/RunEvalSuites/
// RunLoadSuites may pass a per-call progress func which takes precedence
// over this default.
func WithProgress(fn func(completed, total int)) Option {
	return func(r *Runner) {
		r.progress = fn
	}
}

// WithContext installs a runner-wide [suite.ContextDecorator]. The
// decorator is applied to the ctx handed to environment hooks and to
// every suite that does not declare its own decorator via
// [suite.TestSuiteRun.Decorate] / [suite.EvalSuiteRun.Decorate]. Suite-level
// decorators fully replace the runner-level one for that suite; there is
// no chaining.
//
// The runner never inspects the values a decorator attaches; it only
// propagates them. Callers use this seam to stamp caller identity, auth
// headers, tracing state, or any other request-scoped data.
//
// The decorator may be invoked multiple times per LRO — once for
// environment setup/teardown and once per suite that inherits the
// runner-level default — so it should be cheap and free of observable
// side effects. Callers that need to perform expensive per-LRO work
// (minting a token, opening a connection) should do it once at
// construction time and let the decorator only attach the pre-built
// value to the context.
func WithContext(fn suite.ContextDecorator) Option {
	return func(r *Runner) {
		r.decorate = fn
	}
}

// WithAbortOnSLOFailure enables mid-run cancellation when any declared SLO
// fails on partial metrics (checked every 2s inside the generator).
func WithAbortOnSLOFailure() Option {
	return func(r *Runner) {
		r.abortOnSLOFailure = true
	}
}

// baseCtx applies the runner-level decorator, if any. It is used for env
// hooks and as the fallback for suites without their own decorator.
func (r *Runner) baseCtx(ctx context.Context) context.Context {
	if r == nil || r.decorate == nil {
		return ctx
	}
	return r.decorate(ctx)
}

// outgoingContext returns the ctx passed to a suite's setup, cases, and
// teardown. Suite-level decorator overrides runner-level; both nil means
// ctx is returned unchanged.
func (r *Runner) outgoingContext(ctx context.Context, suiteDecorate suite.ContextDecorator) context.Context {
	if suiteDecorate != nil {
		return suiteDecorate(ctx)
	}
	return r.baseCtx(ctx)
}

// RunTestSuites executes test suite runs sequentially and returns one SuiteResult per suite.
func (r *Runner) RunTestSuites(ctx context.Context, runs []suite.TestSuiteRun, progress func(completed, total int)) ([]execution.SuiteResult, error) {
	if r == nil {
		return nil, ErrNilRunner{}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	envTeardown, err := setupEnvironments(r.baseCtx(ctx), collectTestEnvironmentNames(runs))
	if err != nil {
		return r.testRunsEnvironmentSetupFailed(runs, err, progress), nil
	}
	defer envTeardown()

	total := suite.TotalTestCases(runs)
	completed := 0
	notify := func() {
		if progress != nil {
			progress(completed, total)
		} else if r.progress != nil {
			r.progress(completed, total)
		}
	}

	out := make([]execution.SuiteResult, 0, len(runs))
	for _, run := range runs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		suiteStart := time.Now()
		runCtx := r.outgoingContext(ctx, run.Decorate)
		cases := make([]execution.CaseResult, 0, len(run.Cases))

		setupOK := true
		if run.Setup != nil {
			if err := run.Setup(runCtx); err != nil {
				setupOK = false
				for _, c := range run.Cases {
					cases = append(cases, *result.SetupErrorResult(c.Name(), err))
					completed++
					notify()
				}
			}
		}

		if setupOK {
			stopped := false
			var stoppedBy string
			for _, c := range run.Cases {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if stopped {
					cases = append(cases, *result.SkippedResult(c.Name(), skipReason(stoppedBy)))
					completed++
					notify()
					continue
				}
				caseStart := time.Now()
				caseResult := runTestCaseWithRecovery(runCtx, c)
				if caseResult == nil {
					caseResult = result.SetupErrorResult(c.Name(), ErrNilResult{})
				}
				caseResult.Duration = time.Since(caseStart)
				cases = append(cases, *caseResult)
				completed++
				notify()
				if run.StopOnFailure && caseResult.Status != evalspb.Status_PASSED {
					stopped = true
					stoppedBy = caseResult.Name
				}
			}
			if run.Teardown != nil {
				_ = run.Teardown(runCtx)
			}
		}

		suiteEnd := time.Now()
		out = append(out, execution.SuiteResult{
			SuiteName: suiteNameFromRun(run),
			Cases:     cases,
			StartTime: suiteStart,
			EndTime:   suiteEnd,
		})
	}
	return out, nil
}

// RunEvalSuites executes eval suite runs sequentially and returns one SuiteResult per suite.
func (r *Runner) RunEvalSuites(ctx context.Context, runs []suite.EvalSuiteRun, progress func(completed, total int)) ([]execution.SuiteResult, error) {
	if r == nil {
		return nil, ErrNilRunner{}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	envTeardown, err := setupEnvironments(r.baseCtx(ctx), collectEvalEnvironmentNames(runs))
	if err != nil {
		return r.evalRunsEnvironmentSetupFailed(runs, err, progress), nil
	}
	defer envTeardown()

	total := suite.TotalEvalCases(runs)
	completed := 0
	notify := func() {
		if progress != nil {
			progress(completed, total)
		} else if r.progress != nil {
			r.progress(completed, total)
		}
	}

	out := make([]execution.SuiteResult, 0, len(runs))
	for _, run := range runs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		suiteStart := time.Now()
		runCtx := r.outgoingContext(ctx, run.Decorate)
		cases := make([]execution.CaseResult, 0, len(run.Cases))

		setupOK := true
		if run.Setup != nil {
			if err := run.Setup(runCtx); err != nil {
				setupOK = false
				for _, c := range run.Cases {
					cases = append(cases, *result.EvalSetupErrorResult(c.Name(), err))
					completed++
					notify()
				}
			}
		}

		if setupOK {
			stopped := false
			var stoppedBy string
			for _, c := range run.Cases {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if stopped {
					cases = append(cases, *result.EvalSkippedResult(c.Name(), skipReason(stoppedBy)))
					completed++
					notify()
					continue
				}
				caseStart := time.Now()
				caseResult := runEvalCaseWithRecovery(runCtx, c)
				if caseResult == nil {
					caseResult = result.EvalSetupErrorResult(c.Name(), ErrNilResult{})
				}
				caseResult.Duration = time.Since(caseStart)
				cases = append(cases, *caseResult)
				completed++
				notify()
				if run.StopOnFailure && caseResult.Status != evalspb.Status_PASSED {
					stopped = true
					stoppedBy = caseResult.Name
				}
			}
			if run.Teardown != nil {
				_ = run.Teardown(runCtx)
			}
		}

		suiteEnd := time.Now()
		out = append(out, execution.SuiteResult{
			SuiteName: suiteNameFromEvalRun(run),
			Cases:     cases,
			StartTime: suiteStart,
			EndTime:   suiteEnd,
		})
	}
	return out, nil
}

// LoadProfileResolver returns the resolved Profile to use for a given
// suite and mode. The runner is deliberately generic — it does not know
// about the framework's default profile table, so callers supply this seam.
// Return (profile, true) to run the case; return (_, false) to record the
// case as FAILED with a "profile unresolved" reason and keep going.
type LoadProfileResolver func(run suite.LoadSuiteRun, mode evalspb.RunLoadTestRequest_Mode) (loadgen.Profile, bool)

// RunLoadSuites executes load suite runs sequentially, one load case at a
// time. Cases inside a suite are always sequential — concurrent load
// windows against different targets would contaminate each other.
func (r *Runner) RunLoadSuites(
	ctx context.Context,
	runs []suite.LoadSuiteRun,
	mode evalspb.RunLoadTestRequest_Mode,
	resolve LoadProfileResolver,
	progress func(completed, total int),
) ([]execution.LoadSuiteResult, error) {
	if r == nil {
		return nil, ErrNilRunner{}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if resolve == nil {
		return nil, ErrNilLoadProfileResolver{}
	}

	envTeardown, err := setupEnvironments(r.baseCtx(ctx), collectLoadEnvironmentNames(runs))
	if err != nil {
		return r.loadRunsEnvironmentSetupFailed(runs, err, progress), nil
	}
	defer envTeardown()

	total := suite.TotalLoadCases(runs)
	completed := 0
	notify := func() {
		if progress != nil {
			progress(completed, total)
		} else if r.progress != nil {
			r.progress(completed, total)
		}
	}

	out := make([]execution.LoadSuiteResult, 0, len(runs))
	for _, run := range runs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		suiteStart := time.Now()
		runCtx := r.outgoingContext(ctx, nil)
		cases := make([]execution.LoadCaseResult, 0, len(run.Cases))

		profile, profileOK := resolve(run, mode)

		setupOK := true
		if run.Setup != nil {
			if err := run.Setup(runCtx); err != nil {
				setupOK = false
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
			}
		}

		if setupOK {
			runCtx, closeInfra, infraErr := attachInfraClient(runCtx, run.CloudRun, run.Spanner)
			if infraErr != nil {
				for _, c := range run.Cases {
					cases = append(cases, execution.LoadCaseResult{
						Name:   c.Name(),
						Status: evalspb.Status_FAILED,
						Checks: []execution.SloCheckResult{{
							ID:      result.SetupErrorCheckName,
							Status:  evalspb.Status_FAILED,
							Message: infraErr.Error(),
						}},
					})
					completed++
					notify()
				}
			} else {
				for _, c := range run.Cases {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					if !profileOK {
						cases = append(cases, execution.LoadCaseResult{
							Name:   c.Name(),
							Status: evalspb.Status_FAILED,
							Checks: []execution.SloCheckResult{{
								ID:      "profile",
								Status:  evalspb.Status_FAILED,
								Message: fmt.Sprintf("no profile resolved for mode %v", mode),
							}},
						})
						completed++
						notify()
						continue
					}
					caseCtx := runCtx
					if r.abortOnSLOFailure {
						caseCtx = loadgen.ContextWithAbortOnSLOFailure(runCtx)
					}
					caseResult := runLoadCaseWithRecovery(caseCtx, c, mode, profile)
					if caseResult == nil {
						caseResult = &execution.LoadCaseResult{
							Name:   c.Name(),
							Status: evalspb.Status_FAILED,
						}
					}
					cases = append(cases, *caseResult)
					completed++
					notify()
				}
				closeInfra()
			}
			if run.Teardown != nil {
				_ = run.Teardown(runCtx)
			}
		}

		suiteEnd := time.Now()
		out = append(out, execution.LoadSuiteResult{
			SuiteName: suiteNameFromLoadRun(run),
			Cases:     cases,
			StartTime: suiteStart,
			EndTime:   suiteEnd,
		})
	}
	return out, nil
}

// runLoadCaseWithRecovery invokes a load case with panic recovery.
func runLoadCaseWithRecovery(ctx context.Context, c suite.LoadCase, mode evalspb.RunLoadTestRequest_Mode, profile loadgen.Profile) (r *execution.LoadCaseResult) {
	defer func() {
		if v := recover(); v != nil {
			panicErr := ErrCasePanic{Value: v, Stack: string(debug.Stack())}
			r = &execution.LoadCaseResult{
				Name:   c.Name(),
				Status: evalspb.Status_FAILED,
				Checks: []execution.SloCheckResult{{
					ID:      result.CaseErrorCheckName,
					Status:  evalspb.Status_FAILED,
					Message: panicErr.Error(),
				}},
			}
		}
	}()
	return c.Run(ctx, mode, profile)
}

// runTestCaseWithRecovery invokes c.Run and converts panics to a failed case
// result so a single bad case cannot take down the batch.
func runTestCaseWithRecovery(ctx context.Context, c suite.TestCase) (r *execution.CaseResult) {
	defer func() {
		if v := recover(); v != nil {
			r = result.CaseErrorResult(c.Name(), ErrCasePanic{Value: v, Stack: string(debug.Stack())})
		}
	}()
	return c.Run(ctx)
}

// runEvalCaseWithRecovery is [runTestCaseWithRecovery] for eval cases; panics
// surface as failed metrics on the case result.
func runEvalCaseWithRecovery(ctx context.Context, c suite.EvalCase) (r *execution.CaseResult) {
	defer func() {
		if v := recover(); v != nil {
			r = result.EvalCaseErrorResult(c.Name(), ErrCasePanic{Value: v, Stack: string(debug.Stack())})
		}
	}()
	return c.Run(ctx)
}

// skipReason builds the NOT_EVALUATED message when StopOnFailure halts a suite.
func skipReason(failedName string) string {
	if failedName == "" {
		return "preceding case failed"
	}
	return fmt.Sprintf("preceding case %q failed", failedName)
}

// suiteNameFromRun derives the suite name from TestSuiteRun.Name or the first case prefix.
func suiteNameFromRun(run suite.TestSuiteRun) string {
	if run.Name != "" {
		return run.Name
	}
	if len(run.Cases) == 0 {
		return ""
	}
	return suitePrefix(run.Cases[0].Name())
}

// suiteNameFromEvalRun is [suiteNameFromRun] for agent-eval suite runs.
func suiteNameFromEvalRun(run suite.EvalSuiteRun) string {
	if run.Name != "" {
		return run.Name
	}
	if len(run.Cases) == 0 {
		return ""
	}
	return suitePrefix(run.Cases[0].Name())
}

// suiteNameFromLoadRun is [suiteNameFromRun] for load-test suite runs.
func suiteNameFromLoadRun(run suite.LoadSuiteRun) string {
	if run.Name != "" {
		return run.Name
	}
	if len(run.Cases) == 0 {
		return ""
	}
	return suitePrefix(run.Cases[0].Name())
}

// RollupLoadSuiteStatus returns the rolled-up status for a load suite result.
func RollupLoadSuiteStatus(sr execution.LoadSuiteResult) evalspb.Status {
	statuses := make([]evalspb.Status, len(sr.Cases))
	for i, c := range sr.Cases {
		statuses[i] = c.Status
	}
	return result.RollupRunStatus(statuses)
}

// suitePrefix returns the suite segment of a qualified "suite.case" name.
func suitePrefix(qualified string) string {
	suite, _, ok := strings.Cut(qualified, ".")
	if !ok {
		return qualified
	}
	return suite
}

// caseStatuses extracts per-case statuses for suite-level rollup.
func caseStatuses(cases []execution.CaseResult) []evalspb.Status {
	statuses := make([]evalspb.Status, len(cases))
	for i, s := range cases {
		statuses[i] = s.Status
	}
	return statuses
}

// RollupSuiteStatus returns the rolled-up status for a suite result.
func RollupSuiteStatus(sr execution.SuiteResult) evalspb.Status {
	return result.RollupRunStatus(caseStatuses(sr.Cases))
}
