package runner

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/auth"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/suite"
	iam "go.alis.build/iam/v3"
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
	progress func(completed, total int)
	identity *iam.Identity
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

// WithIdentity sets a fallback caller identity for suites that don't
// declare one via [suite.TestSuiteRun.Identity] or [suite.EvalSuiteRun.Identity].
// The identity is attached to the outgoing context via [auth.Outgoing]
// for every case in the suite.
func WithIdentity(identity *iam.Identity) Option {
	return func(r *Runner) {
		r.identity = identity
	}
}

func (r *Runner) outgoingContext(ctx context.Context, suiteIdentity *iam.Identity) context.Context {
	if r == nil {
		return ctx
	}

	id := suiteIdentity
	if id == nil {
		id = r.identity
	}
	return auth.Outgoing(ctx, id)
}

// RunTestSuites executes test suite runs sequentially and returns one SuiteResult per suite.
func (r *Runner) RunTestSuites(ctx context.Context, runs []suite.TestSuiteRun, progress func(completed, total int)) ([]execution.SuiteResult, error) {
	if r == nil {
		return nil, ErrNilRunner{}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	envTeardown, err := setupEnvironments(ctx, collectTestEnvironmentNames(runs))
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
		runCtx := r.outgoingContext(ctx, run.Identity)
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

	envTeardown, err := setupEnvironments(ctx, collectEvalEnvironmentNames(runs))
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
		runCtx := r.outgoingContext(ctx, run.Identity)
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
		return nil, fmt.Errorf("runner: nil load profile resolver")
	}

	envTeardown, err := setupEnvironments(ctx, collectLoadEnvironmentNames(runs))
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
				caseResult := runLoadCaseWithRecovery(runCtx, c, mode, profile)
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
			r = &execution.LoadCaseResult{
				Name:   c.Name(),
				Status: evalspb.Status_FAILED,
				Checks: []execution.SloCheckResult{{
					ID:      result.CaseErrorCheckName,
					Status:  evalspb.Status_FAILED,
					Message: fmt.Sprintf("panic: %v\n%s", v, debug.Stack()),
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
			r = result.CaseErrorResult(c.Name(), fmt.Errorf("panic: %v\n%s", v, debug.Stack()))
		}
	}()
	return c.Run(ctx)
}

// runEvalCaseWithRecovery is [runTestCaseWithRecovery] for eval cases; panics
// surface as failed metrics on the case result.
func runEvalCaseWithRecovery(ctx context.Context, c suite.EvalCase) (r *execution.CaseResult) {
	defer func() {
		if v := recover(); v != nil {
			r = result.EvalCaseErrorResult(c.Name(), fmt.Errorf("panic: %v\n%s", v, debug.Stack()))
		}
	}()
	return c.Run(ctx)
}

func skipReason(failedName string) string {
	if failedName == "" {
		return "preceding case failed"
	}
	return fmt.Sprintf("preceding case %q failed", failedName)
}

func suiteNameFromRun(run suite.TestSuiteRun) string {
	if run.Name != "" {
		return run.Name
	}
	if len(run.Cases) == 0 {
		return ""
	}
	return suitePrefix(run.Cases[0].Name())
}

func suiteNameFromEvalRun(run suite.EvalSuiteRun) string {
	if run.Name != "" {
		return run.Name
	}
	if len(run.Cases) == 0 {
		return ""
	}
	return suitePrefix(run.Cases[0].Name())
}

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

func suitePrefix(qualified string) string {
	suite, _, ok := strings.Cut(qualified, ".")
	if !ok {
		return qualified
	}
	return suite
}

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
