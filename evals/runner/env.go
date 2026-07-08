package runner

import (
	"context"

	"go.alis.build/evals/env"
	"go.alis.build/evals/suite"
)

func collectTestEnvironmentNames(runs []suite.TestSuiteRun) []string {
	return collectEnvironmentNames(func(i int) []string { return runs[i].Environments }, len(runs))
}

func collectEvalEnvironmentNames(runs []suite.EvalSuiteRun) []string {
	return collectEnvironmentNames(func(i int) []string { return runs[i].Environments }, len(runs))
}

func collectLoadEnvironmentNames(runs []suite.LoadSuiteRun) []string {
	return collectEnvironmentNames(func(i int) []string { return runs[i].Environments }, len(runs))
}

func collectEnvironmentNames(environments func(int) []string, count int) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for i := 0; i < count; i++ {
		for _, name := range environments(i) {
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

func setupEnvironments(ctx context.Context, names []string) (func(), error) {
	if len(names) == 0 {
		return func() {}, nil
	}

	completed := make([]string, 0, len(names))
	for _, name := range names {
		e := env.Get(name)
		if e == nil {
			teardownCompletedEnvironments(ctx, completed)
			return func() {}, env.ErrNotRegistered{Name: name}
		}
		if hook := e.Setup(); hook != nil {
			if err := hook(ctx); err != nil {
				teardownCompletedEnvironments(ctx, completed)
				return func() {}, env.NewSetupFailed(name, err)
			}
		}
		completed = append(completed, name)
	}

	return func() { teardownCompletedEnvironments(ctx, completed) }, nil
}

func teardownCompletedEnvironments(ctx context.Context, completed []string) {
	for i := len(completed) - 1; i >= 0; i-- {
		if e := env.Get(completed[i]); e != nil {
			if hook := e.Teardown(); hook != nil {
				_ = hook(ctx)
			}
		}
	}
}
