package runner

import (
	"context"
	"time"

	"go.alis.build/alog"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/internal/result"
	"go.alis.build/evals/suite"
)

const suiteTeardownTimeout = 30 * time.Second

func runSuiteTeardown(ctx context.Context, hook suite.SuiteHook) error {
	if hook == nil {
		return nil
	}
	teardownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), suiteTeardownTimeout)
	defer cancel()
	if err := hook(teardownCtx); err != nil {
		alog.Errorf(teardownCtx, "suite teardown failed: %v", err)
		return err
	}
	return nil
}

func applyTestTeardownFailure(cases []execution.CaseResult, err error) {
	result.ApplyTeardownFailureToCaseResults(cases, err)
}

func applyEvalTeardownFailure(cases []execution.CaseResult, err error) {
	result.ApplyTeardownFailureToCaseResults(cases, err)
}

func applyLoadTeardownFailure(cases []execution.LoadCaseResult, err error) {
	result.ApplyTeardownFailureToLoadCases(cases, err)
}

func applyInfraObserveTeardownFailure(cases []execution.InfraObserveCaseResult, err error) {
	result.ApplyTeardownFailureToInfraCases(cases, err)
}
