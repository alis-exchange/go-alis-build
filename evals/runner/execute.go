package runner

import (
	"context"
	"sync"
)

// ExecuteOptions configures the generic case-execution loop shared by all
// Run*Suites methods. Callers assemble suite envelopes around the returned
// case slice plus wall-clock times.
type ExecuteOptions struct {
	// Decorate transforms the context handed to each case body. Nil means the
	// incoming context is used unchanged.
	Decorate func(context.Context) context.Context
	// StopOnFailure skips remaining cases with NOT_EVALUATED once any case
	// ends non-PASSED. Only honoured in sequential mode.
	StopOnFailure bool
	// Sequential runs cases one at a time. When false, cases run concurrently
	// up to Concurrency goroutines and output order matches input order.
	Sequential bool
	// OnCaseComplete fires once per finished case as (completed, total).
	OnCaseComplete func(completed, total int)
	// Concurrency bounds concurrent case goroutines when Sequential is false.
	// Non-positive values use [defaultInfraObserveConcurrency].
	Concurrency int
}

// ExecuteHooks supplies case-kind callbacks [Execute] uses for naming,
// pass checks, and synthetic skip/cancel results. Suite runners wire these
// once per kind via package-private helpers in execute_hooks.go.
type ExecuteHooks[C, CaseR any] struct {
	CaseName        func(C) string
	IsPassed        func(CaseR) bool
	SkippedResult   func(name, reason string) CaseR
	CancelledResult func(name string) CaseR
}

// Execute runs a homogeneous case list with shared lifecycle semantics.
// CaseR is the per-case result type (for example [execution.CaseResult]).
//
// Execute owns cancel mid-suite, StopOnFailure skip, and progress notification
// per completed case. In concurrent mode, work already running may finish after
// cancellation; selected cases that have not started receive CancelledResult.
// Panic recovery and nil-result handling belong in runCase.
func Execute[C, CaseR any](
	ctx context.Context,
	cases []C,
	runCase func(context.Context, C) CaseR,
	opts ExecuteOptions,
	hooks ExecuteHooks[C, CaseR],
) ([]CaseR, error) {
	if len(cases) == 0 {
		return nil, nil
	}
	runCtx := ctx
	if opts.Decorate != nil {
		runCtx = opts.Decorate(ctx)
	}
	total := len(cases)
	if opts.Sequential {
		return executeSequential(runCtx, cases, runCase, opts, hooks, total)
	}
	return executeConcurrent(runCtx, cases, runCase, opts, hooks, total)
}

func executeSequential[C, CaseR any](
	ctx context.Context,
	cases []C,
	runCase func(context.Context, C) CaseR,
	opts ExecuteOptions,
	hooks ExecuteHooks[C, CaseR],
	total int,
) ([]CaseR, error) {
	out := make([]CaseR, 0, len(cases))
	completed := 0
	notify := newExecuteNotifier(opts.OnCaseComplete, total)
	stopped := false
	var stoppedBy string

	for _, c := range cases {
		name := hooks.CaseName(c)
		if err := ctx.Err(); err != nil {
			for _, rem := range cases[len(out):] {
				out = append(out, hooks.CancelledResult(hooks.CaseName(rem)))
				completed++
				notify(completed)
			}
			return out, err
		}
		if stopped {
			out = append(out, hooks.SkippedResult(name, skipReason(stoppedBy)))
			completed++
			notify(completed)
			continue
		}
		caseResult := runCase(ctx, c)
		out = append(out, caseResult)
		completed++
		notify(completed)
		if opts.StopOnFailure && !hooks.IsPassed(caseResult) {
			stopped = true
			stoppedBy = name
		}
	}
	return out, nil
}

func executeConcurrent[C, CaseR any](
	ctx context.Context,
	cases []C,
	runCase func(context.Context, C) CaseR,
	opts ExecuteOptions,
	hooks ExecuteHooks[C, CaseR],
	total int,
) ([]CaseR, error) {
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = defaultInfraObserveConcurrency
	}

	slots := make([]CaseR, len(cases))
	completed := 0
	var progressMu sync.Mutex
	notify := func() {
		progressMu.Lock()
		completed++
		if opts.OnCaseComplete != nil {
			opts.OnCaseComplete(completed, total)
		}
		progressMu.Unlock()
	}

	if concurrency > len(cases) {
		concurrency = len(cases)
	}
	type caseJob struct {
		index int
		value C
	}
	jobs := make(chan caseJob)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if ctx.Err() != nil {
					slots[job.index] = hooks.CancelledResult(hooks.CaseName(job.value))
					notify()
					continue
				}
				slots[job.index] = runCase(ctx, job.value)
				notify()
			}
		}()
	}

	for i, c := range cases {
		select {
		case jobs <- caseJob{index: i, value: c}:
		case <-ctx.Done():
			for j := i; j < len(cases); j++ {
				slots[j] = hooks.CancelledResult(hooks.CaseName(cases[j]))
				notify()
			}
			close(jobs)
			wg.Wait()
			return slots, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	return slots, ctx.Err()
}

type executeNotifier func(completed int)

func newExecuteNotifier(onComplete func(completed, total int), total int) executeNotifier {
	if onComplete == nil {
		return func(int) {}
	}
	return func(completed int) {
		onComplete(completed, total)
	}
}
