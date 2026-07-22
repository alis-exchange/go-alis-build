package harness

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"go.alis.build/alog"
	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/mapper"
	"go.alis.build/evals/report"
	"go.alis.build/evals/runner"
	"go.alis.build/evals/suite"
)

// RunMeta carries request-scoped identifiers stamped onto each mapped evalspb.Run.
type RunMeta struct {
	Operation string
	RunID     string
	BatchID   string // optional; integration, load, and infra observation batches
}

// SuiteRunner executes a batch and returns one result per suite.
type SuiteRunner[C, R any] func(ctx context.Context, cases []C) ([]R, error)

// BatchOptions configures optional batch-helper behaviour.
type BatchOptions struct {
	// Progress is called after each case completes.
	Progress func(completed, total int)
	// SuiteProgress is called after each suite has been mapped, reported,
	// and recorded in the returned run names.
	SuiteProgress func(completed, total int)
}

func recordSuite(names *[]string, name string, completed *int, total int, opts BatchOptions, mu *sync.Mutex) {
	mu.Lock()
	if name != "" {
		*names = append(*names, name)
	}
	*completed = *completed + 1
	current := *completed
	mu.Unlock()
	if opts.SuiteProgress != nil {
		opts.SuiteProgress(current, total)
	}
}

// RunSuite executes cases via run, maps each suite result, and reports best-effort.
// The cases argument is forwarded to run unchanged (for example a []suite.TestSuiteRun
// batch passed to a closure that calls [runner.Runner.RunTestSuites]).
// Reporter errors are logged and do not fail RunSuite. A nil reporter maps
// only, matching nil TestServiceServer.Reporter behaviour.
func RunSuite[C, R any](
	ctx context.Context,
	cases []C,
	run SuiteRunner[C, R],
	mapRun func(R, RunMeta) *evalspb.Run,
	reporter report.Reporter,
	meta RunMeta,
) ([]string, error) {
	results, err := run(ctx, cases)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(results))
	for _, result := range results {
		suiteMeta := meta
		if suiteMeta.RunID == "" || len(results) > 1 {
			suiteMeta.RunID = newRunID()
		}
		name := mapReport(ctx, result, suiteMeta, mapRun, reporter)
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func mapReport[R any](
	ctx context.Context,
	result R,
	meta RunMeta,
	mapRun func(R, RunMeta) *evalspb.Run,
	reporter report.Reporter,
) string {
	wire := mapRun(result, meta)
	if wire == nil {
		return ""
	}
	if reporter != nil {
		if err := reporter.ReportRun(ctx, wire); err != nil {
			alog.Errorf(ctx, "harness: report %s: %v", wire.GetName(), err)
		}
	}
	return wire.GetName()
}

func newRunID() string {
	return uuid.NewString()
}

// RunIntegrationBatch executes integration suites, maps each result, and reports.
func RunIntegrationBatch(
	ctx context.Context,
	r *runner.Runner,
	runs []suite.TestSuiteRun,
	meta RunMeta,
	reporter report.Reporter,
	opts BatchOptions,
) ([]string, error) {
	var (
		names     []string
		completed int
		mu        sync.Mutex
	)
	_, err := r.RunTestSuites(ctx, runs, opts.Progress, func(ctx context.Context, sr execution.SuiteResult) error {
		suiteMeta := meta
		suiteMeta.RunID = newRunID()
		name := mapReport(ctx, sr, suiteMeta, func(sr execution.SuiteResult, m RunMeta) *evalspb.Run {
			return mapper.IntegrationRun(sr, m.Operation, m.RunID, m.BatchID)
		}, reporter)
		recordSuite(&names, name, &completed, len(runs), opts, &mu)
		return nil
	})
	return names, err
}

// RunEvalBatch executes agent-eval suites, maps each result, and reports.
func RunEvalBatch(
	ctx context.Context,
	r *runner.Runner,
	runs []suite.EvalSuiteRun,
	meta RunMeta,
	reporter report.Reporter,
	opts BatchOptions,
) ([]string, error) {
	var (
		names     []string
		completed int
		mu        sync.Mutex
	)
	_, err := r.RunEvalSuites(ctx, runs, opts.Progress, func(ctx context.Context, sr execution.SuiteResult) error {
		suiteMeta := meta
		suiteMeta.RunID = newRunID()
		name := mapReport(ctx, sr, suiteMeta, func(sr execution.SuiteResult, m RunMeta) *evalspb.Run {
			return mapper.AgentEvalRun(sr, m.Operation, m.RunID)
		}, reporter)
		recordSuite(&names, name, &completed, len(runs), opts, &mu)
		return nil
	})
	return names, err
}

// RunLoadBatch executes load suites, maps each result, and reports.
func RunLoadBatch(
	ctx context.Context,
	r *runner.Runner,
	runs []suite.LoadSuiteRun,
	mode evalspb.RunLoadTestRequest_Mode,
	resolve runner.LoadProfileResolver,
	meta RunMeta,
	reporter report.Reporter,
	opts BatchOptions,
) ([]string, error) {
	var (
		names     []string
		completed int
		mu        sync.Mutex
	)
	_, err := r.RunLoadSuites(ctx, runs, mode, resolve, opts.Progress, func(ctx context.Context, sr execution.LoadSuiteResult) error {
		suiteMeta := meta
		suiteMeta.RunID = newRunID()
		name := mapReport(ctx, sr, suiteMeta, func(sr execution.LoadSuiteResult, m RunMeta) *evalspb.Run {
			return mapper.LoadRun(sr, m.Operation, m.RunID, m.BatchID)
		}, reporter)
		recordSuite(&names, name, &completed, len(runs), opts, &mu)
		return nil
	})
	return names, err
}

// RunInfraObserveBatch executes infra-observe suites, maps each result, and reports.
func RunInfraObserveBatch(
	ctx context.Context,
	r *runner.Runner,
	runs []suite.InfraObserveSuiteRun,
	params runner.InfraObserveRunParams,
	meta RunMeta,
	reporter report.Reporter,
	opts BatchOptions,
) ([]string, error) {
	var (
		names     []string
		completed int
		mu        sync.Mutex
	)
	_, err := r.RunInfraObserveSuites(ctx, runs, params, opts.Progress, func(ctx context.Context, sr execution.InfraObserveSuiteResult) error {
		suiteMeta := meta
		suiteMeta.RunID = newRunID()
		name := mapReport(ctx, sr, suiteMeta, func(sr execution.InfraObserveSuiteResult, m RunMeta) *evalspb.Run {
			return mapper.InfraObserveRun(sr, m.Operation, m.RunID, m.BatchID)
		}, reporter)
		recordSuite(&names, name, &completed, len(runs), opts, &mu)
		return nil
	})
	return names, err
}
