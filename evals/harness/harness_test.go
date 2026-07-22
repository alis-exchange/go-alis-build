package harness

import (
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/mapper"
	"go.alis.build/evals/report"
	"go.alis.build/evals/runner"
	"go.alis.build/evals/suite"
)

type recordingReporter struct {
	runs []*evalspb.Run
	err  error
}

func (r *recordingReporter) ReportRun(_ context.Context, run *evalspb.Run) error {
	if run != nil {
		r.runs = append(r.runs, run)
	}
	return r.err
}

func stubSuiteResult(name string) execution.SuiteResult {
	now := time.Now()
	return execution.SuiteResult{
		SuiteName: name,
		StartTime: now.Add(-time.Second),
		EndTime:   now,
		Cases: []execution.CaseResult{{
			Name:   name + ".case",
			Status: evalspb.Status_PASSED,
		}},
	}
}

func TestRunSuite_mapsAndReportsEachResult(t *testing.T) {
	t.Parallel()

	rec := &recordingReporter{}
	meta := RunMeta{Operation: "operations/op-1", BatchID: "batch-1"}

	names, err := RunSuite(context.Background(), []string{"ignored"},
		func(_ context.Context, _ []string) ([]execution.SuiteResult, error) {
			return []execution.SuiteResult{
				stubSuiteResult("suite-a"),
				stubSuiteResult("suite-b"),
			}, nil
		},
		func(sr execution.SuiteResult, m RunMeta) *evalspb.Run {
			return mapper.IntegrationRun(sr, m.Operation, m.RunID, m.BatchID)
		},
		rec,
		meta,
	)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("names = %v, want 2", names)
	}
	if len(rec.runs) != 2 {
		t.Fatalf("reported runs = %d, want 2", len(rec.runs))
	}
	for i, wire := range rec.runs {
		if wire.GetOperation() != meta.Operation {
			t.Fatalf("run[%d].Operation = %q, want %q", i, wire.GetOperation(), meta.Operation)
		}
		if wire.GetBatchId() != meta.BatchID {
			t.Fatalf("run[%d].BatchId = %q, want %q", i, wire.GetBatchId(), meta.BatchID)
		}
		if wire.GetName() != names[i] {
			t.Fatalf("run[%d].Name = %q, recorded name = %q", i, wire.GetName(), names[i])
		}
	}
}

func TestRunSuite_nilReporterSkipsIO(t *testing.T) {
	t.Parallel()

	names, err := RunSuite(context.Background(), nil,
		func(_ context.Context, _ []string) ([]execution.SuiteResult, error) {
			return []execution.SuiteResult{stubSuiteResult("solo")}, nil
		},
		func(sr execution.SuiteResult, m RunMeta) *evalspb.Run {
			return mapper.IntegrationRun(sr, m.Operation, m.RunID, m.BatchID)
		},
		nil,
		RunMeta{Operation: "operations/op-1", RunID: "fixed-run"},
	)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if len(names) != 1 || names[0] != "runs/fixed-run" {
		t.Fatalf("names = %v, want [runs/fixed-run]", names)
	}
}

func TestRunSuite_reporterErrorStillReturnsNames(t *testing.T) {
	t.Parallel()

	rec := &recordingReporter{err: errors.New("sink unavailable")}
	names, err := RunSuite(context.Background(), nil,
		func(_ context.Context, _ []string) ([]execution.SuiteResult, error) {
			return []execution.SuiteResult{stubSuiteResult("suite-a")}, nil
		},
		func(sr execution.SuiteResult, m RunMeta) *evalspb.Run {
			return mapper.IntegrationRun(sr, m.Operation, m.RunID, m.BatchID)
		},
		rec,
		RunMeta{Operation: "operations/op-1"},
	)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("names = %v, want one name despite reporter error", names)
	}
	if len(rec.runs) != 1 {
		t.Fatalf("reporter calls = %d, want 1", len(rec.runs))
	}
}

func TestRunSuite_executorError(t *testing.T) {
	t.Parallel()

	rec := &recordingReporter{}
	_, err := RunSuite(context.Background(), nil,
		func(context.Context, []string) ([]execution.SuiteResult, error) {
			return nil, errors.New("execute failed")
		},
		func(sr execution.SuiteResult, m RunMeta) *evalspb.Run {
			return mapper.IntegrationRun(sr, m.Operation, m.RunID, m.BatchID)
		},
		rec,
		RunMeta{Operation: "operations/op-1"},
	)
	if err == nil {
		t.Fatal("RunSuite: want executor error")
	}
	if len(rec.runs) != 0 {
		t.Fatalf("reported runs = %d, want 0 on executor failure", len(rec.runs))
	}
}

func TestRunIntegrationBatch_wiresMapperAndReporter(t *testing.T) {
	t.Parallel()

	rec := &recordingReporter{}
	r := runner.New()
	runs := []suite.TestSuiteRun{{
		Cases: []suite.TestCase{
			stubHarnessTestCase{name: "suite.case", status: evalspb.Status_PASSED},
		},
	}}

	names, err := RunIntegrationBatch(context.Background(), r, runs,
		RunMeta{Operation: "operations/test", BatchID: "batch-1"}, rec, BatchOptions{})
	if err != nil {
		t.Fatalf("RunIntegrationBatch: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("names = %v", names)
	}
	if len(rec.runs) != 1 || rec.runs[0].GetType() != evalspb.Run_INTEGRATION_TEST {
		t.Fatalf("reported run = %+v, want INTEGRATION_TEST", rec.runs[0])
	}
}

func TestRunEvalBatch_wiresMapperAndReporter(t *testing.T) {
	t.Parallel()

	rec := &recordingReporter{}
	r := runner.New()
	runs := []suite.EvalSuiteRun{{
		Cases: []suite.EvalCase{
			stubHarnessEvalCase{name: "agent.case", status: evalspb.Status_PASSED},
		},
	}}

	names, err := RunEvalBatch(context.Background(), r, runs,
		RunMeta{Operation: "operations/eval"}, rec, BatchOptions{})
	if err != nil {
		t.Fatalf("RunEvalBatch: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("names = %v", names)
	}
	if len(rec.runs) != 1 || rec.runs[0].GetType() != evalspb.Run_AGENT_EVAL {
		t.Fatalf("reported run = %+v, want AGENT_EVAL", rec.runs[0])
	}
}

func TestRunLoadBatch_wiresMapperAndReporter(t *testing.T) {
	t.Parallel()

	rec := &recordingReporter{}
	r := runner.New()
	runs := []suite.LoadSuiteRun{{
		Cases: []suite.LoadCase{
			stubHarnessLoadCase{name: "load.case", status: evalspb.Status_PASSED},
		},
	}}

	names, err := RunLoadBatch(context.Background(), r, runs, evalspb.RunLoadTestRequest_MINIMAL,
		func(suite.LoadSuiteRun, evalspb.RunLoadTestRequest_Mode) (loadgen.Profile, bool) {
			return loadgen.Profile{}, true
		},
		RunMeta{Operation: "operations/load", BatchID: "batch-1"},
		rec,
		BatchOptions{},
	)
	if err != nil {
		t.Fatalf("RunLoadBatch: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("names = %v", names)
	}
	if len(rec.runs) != 1 || rec.runs[0].GetType() != evalspb.Run_LOAD_TEST {
		t.Fatalf("reported run = %+v, want LOAD_TEST", rec.runs[0])
	}
}

func TestRunInfraObserveBatch_wiresMapperAndReporter(t *testing.T) {
	t.Parallel()

	rec := &recordingReporter{}
	r := runner.New()
	runs := []suite.InfraObserveSuiteRun{{
		Cases: []suite.InfraObserveCase{
			stubHarnessInfraCase{name: "infra.case", status: evalspb.Status_PASSED},
		},
	}}

	names, err := RunInfraObserveBatch(context.Background(), r, runs, runner.InfraObserveRunParams{},
		RunMeta{Operation: "operations/infra", BatchID: "batch-1"}, rec, BatchOptions{})
	if err != nil {
		t.Fatalf("RunInfraObserveBatch: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("names = %v", names)
	}
	if len(rec.runs) != 1 || rec.runs[0].GetType() != evalspb.Run_INFRA_OBSERVATION {
		t.Fatalf("reported run = %+v, want INFRA_OBSERVATION", rec.runs[0])
	}
}

type stubHarnessTestCase struct {
	name   string
	status evalspb.Status
}

func (c stubHarnessTestCase) Name() string { return c.name }

func (c stubHarnessTestCase) Run(context.Context) *execution.CaseResult {
	return &execution.CaseResult{Name: c.name, Status: c.status}
}

type stubHarnessEvalCase struct {
	name   string
	status evalspb.Status
}

func (c stubHarnessEvalCase) Name() string { return c.name }

func (c stubHarnessEvalCase) Run(context.Context) *execution.CaseResult {
	return &execution.CaseResult{Name: c.name, Status: c.status}
}

type stubHarnessLoadCase struct {
	name   string
	status evalspb.Status
}

func (c stubHarnessLoadCase) Name() string { return c.name }

func (c stubHarnessLoadCase) Run(context.Context, evalspb.RunLoadTestRequest_Mode, loadgen.Profile) *execution.LoadCaseResult {
	return &execution.LoadCaseResult{Name: c.name, Status: c.status}
}

type stubHarnessInfraCase struct {
	name   string
	status evalspb.Status
}

func (c stubHarnessInfraCase) Name() string { return c.name }

func (c stubHarnessInfraCase) Lookback() (time.Duration, bool) { return 0, false }

func (c stubHarnessInfraCase) Run(context.Context, suite.InfraObserveCaseConfig) *execution.InfraObserveCaseResult {
	return &execution.InfraObserveCaseResult{Name: c.name, Status: c.status}
}

var _ report.Reporter = (*recordingReporter)(nil)
