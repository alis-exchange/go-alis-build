package runner

import (
	"context"
	"time"

	"go.alis.build/alog"
	"go.alis.build/evals/env"
	"go.alis.build/evals/suite"
)

const envTeardownTimeout = 30 * time.Second

// collectTestEnvironmentNames deduplicates environment names across integration-test runs.
func collectTestEnvironmentNames(runs []suite.TestSuiteRun) []string {
	return collectEnvironmentNames(func(i int) []string { return runs[i].Environments }, len(runs))
}

// collectEvalEnvironmentNames deduplicates environment names across agent-eval runs.
func collectEvalEnvironmentNames(runs []suite.EvalSuiteRun) []string {
	return collectEnvironmentNames(func(i int) []string { return runs[i].Environments }, len(runs))
}

func firstTestEnvRegistry(runs []suite.TestSuiteRun) *env.Registry {
	if len(runs) > 0 {
		return runs[0].EnvRegistry
	}
	return nil
}

func firstEvalEnvRegistry(runs []suite.EvalSuiteRun) *env.Registry {
	if len(runs) > 0 {
		return runs[0].EnvRegistry
	}
	return nil
}

func firstLoadEnvRegistry(runs []suite.LoadSuiteRun) *env.Registry {
	if len(runs) > 0 {
		return runs[0].EnvRegistry
	}
	return nil
}

func firstInfraObserveEnvRegistry(runs []suite.InfraObserveSuiteRun) *env.Registry {
	if len(runs) > 0 {
		return runs[0].EnvRegistry
	}
	return nil
}

// collectLoadEnvironmentNames deduplicates environment names across load-test runs.
func collectLoadEnvironmentNames(runs []suite.LoadSuiteRun) []string {
	return collectEnvironmentNames(func(i int) []string { return runs[i].Environments }, len(runs))
}

// collectEnvironmentNames walks runs via environments and returns unique names in first-seen order.
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

// setupEnvironments runs Setup hooks for each named environment in order. The
// returned teardown runs completed environments in reverse order on defer.
func setupEnvironments(ctx context.Context, registry *env.Registry, names []string) (func(), error) {
	if len(names) == 0 {
		return func() {}, nil
	}
	if registry == nil {
		registry = env.DefaultRegistry()
	}

	completed := make([]string, 0, len(names))
	for _, name := range names {
		e := registry.Get(name)
		if e == nil {
			teardownCompletedEnvironments(ctx, registry, completed)
			return func() {}, env.ErrNotRegistered{Name: name}
		}
		if hook := e.Setup(); hook != nil {
			if err := hook(ctx); err != nil {
				teardownCompletedEnvironments(ctx, registry, completed)
				return func() {}, env.NewSetupFailed(name, err)
			}
		}
		completed = append(completed, name)
	}

	return func() { teardownCompletedEnvironments(ctx, registry, completed) }, nil
}

// teardownCompletedEnvironments runs Teardown hooks for environments set up so far.
func teardownCompletedEnvironments(ctx context.Context, registry *env.Registry, completed []string) {
	tdCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), envTeardownTimeout)
	defer cancel()
	for i := len(completed) - 1; i >= 0; i-- {
		name := completed[i]
		if e := registry.Get(name); e != nil {
			if hook := e.Teardown(); hook != nil {
				if err := hook(tdCtx); err != nil {
					alog.Errorf(tdCtx, "environment %q teardown failed: %v", name, err)
				}
			}
		}
	}
}
