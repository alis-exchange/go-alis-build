package evals

import (
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/evals/report"
	"go.alis.build/validation"
)

func TestRunAndPublish_usesLazyStandardReporter(t *testing.T) {
	rec := &publicationRecorder{}
	restore := stubStandardReporter(t, rec, nil)
	defer restore()

	s := NewIntegrationSuite("publish-default").
		AddCase("case", func(_ context.Context, v *validation.Validator) {
			v.Custom("ok", true)
		})
	if _, err := s.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if rec.reports != 0 {
		t.Fatalf("Run() reports = %d, want 0", rec.reports)
	}

	run, err := s.RunAndPublish(context.Background())
	if err != nil {
		t.Fatalf("RunAndPublish() error = %v", err)
	}
	if rec.reports != 1 {
		t.Fatalf("RunAndPublish reports = %d, want 1", rec.reports)
	}
	if rec.closed != 1 {
		t.Fatalf("default reporter Close calls = %d, want 1", rec.closed)
	}
	if rec.last != run {
		t.Fatal("reporter did not receive returned run")
	}
}

func TestRunAndPublish_customReporterReplacesDefaultAndIsNotClosed(t *testing.T) {
	defaultRec := &publicationRecorder{}
	restore := stubStandardReporter(t, defaultRec, nil)
	defer restore()
	custom := &publicationRecorder{}

	_, err := NewIntegrationSuite("publish-custom").
		AddCase("case", func(_ context.Context, v *validation.Validator) {
			v.Custom("ok", true)
		}).
		RunAndPublish(context.Background(), WithReporter(custom))
	if err != nil {
		t.Fatalf("RunAndPublish() error = %v", err)
	}
	if defaultRec.reports != 0 {
		t.Fatalf("default reporter reports = %d, want 0", defaultRec.reports)
	}
	if custom.reports != 1 {
		t.Fatalf("custom reports = %d, want 1", custom.reports)
	}
	if custom.closed != 0 {
		t.Fatalf("custom Close calls = %d, want 0", custom.closed)
	}
}

func TestRunAndPublish_acceptsMultiReporterOption(t *testing.T) {
	t.Parallel()

	r1 := &publicationRecorder{}
	r2 := &publicationRecorder{}
	_, err := NewIntegrationSuite("publish-multi").
		AddCase("case", func(_ context.Context, v *validation.Validator) {
			v.Custom("ok", true)
		}).
		RunAndPublish(context.Background(), WithReporter(report.MultiReporter{r1, r2}))
	if err != nil {
		t.Fatalf("RunAndPublish() error = %v", err)
	}
	if r1.reports != 1 || r2.reports != 1 {
		t.Fatalf("multi reports r1=%d r2=%d, want 1 each", r1.reports, r2.reports)
	}
}

func TestRunAndPublish_returnsRunWhenDefaultReporterCreationFails(t *testing.T) {
	sentinel := errors.New("default reporter unavailable")
	restore := stubStandardReporter(t, nil, sentinel)
	defer restore()

	run, err := NewIntegrationSuite("publish-default-error").
		AddCase("case", func(_ context.Context, v *validation.Validator) {
			v.Custom("ok", true)
		}).
		RunAndPublish(context.Background())
	if run == nil {
		t.Fatal("RunAndPublish returned nil run, want materialized run")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("RunAndPublish error = %v, want sentinel", err)
	}
}

func TestRunAndPublish_returnsRunWhenReporterFails(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("sink unavailable")
	rec := &publicationRecorder{err: sentinel}
	run, err := NewIntegrationSuite("publish-error").
		AddCase("case", func(_ context.Context, v *validation.Validator) {
			v.Custom("ok", true)
		}).
		RunAndPublish(context.Background(), WithReporter(rec))
	if run == nil {
		t.Fatal("RunAndPublish returned nil run, want materialized run")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("RunAndPublish error = %v, want sentinel", err)
	}
	if rec.reports != 1 {
		t.Fatalf("report calls = %d, want 1", rec.reports)
	}
}

func TestRunAndPublish_publishesPartialCancelledRun(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "trace-value")
	ctx, cancel := context.WithCancel(ctx)
	rec := &publicationRecorder{contextKey: contextKey{}}
	run, err := NewAgentEvalSuite("publish-cancel").
		AddCase("first", func(_ context.Context, r *AgentEvalResult) {
			r.AddMetric(&evalspb.AgentEvalResults_Case_Metric{Id: "done", Status: evalspb.Status_PASSED})
			cancel()
		}).
		AddCase("second", func(_ context.Context, r *AgentEvalResult) {
			r.AddMetric(&evalspb.AgentEvalResults_Case_Metric{Id: "unexpected", Status: evalspb.Status_PASSED})
		}).
		RunAndPublish(ctx, WithMaxConcurrency(1), WithReporter(rec))
	if run == nil {
		t.Fatal("RunAndPublish returned nil run, want partial run")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunAndPublish error = %v, want context.Canceled", err)
	}
	if rec.reports != 1 {
		t.Fatalf("reports = %d, want partial run published", rec.reports)
	}
	if rec.last != run {
		t.Fatal("reported run differs from returned run")
	}
	if rec.contextValue != "trace-value" {
		t.Fatalf("report context value = %v, want trace-value", rec.contextValue)
	}
}

func TestPublishRun_replacesExpiredExecutionDeadline(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	parent := context.WithValue(context.Background(), contextKey{}, "trace-value")
	ctx, cancel := context.WithDeadline(parent, time.Now().Add(-time.Second))
	defer cancel()

	rec := &publicationRecorder{contextKey: contextKey{}}
	if err := publishRun(ctx, runConfig{reporter: rec}, &evalspb.Run{}); err != nil {
		t.Fatalf("publishRun() error = %v", err)
	}
	if rec.contextErr != nil {
		t.Fatalf("report context error = %v, want nil", rec.contextErr)
	}
	if rec.contextValue != "trace-value" {
		t.Fatalf("report context value = %v, want trace-value", rec.contextValue)
	}
	if time.Until(rec.deadline) < defaultSuitePublishTimeout/2 {
		t.Fatalf("report deadline = %v, want fresh publication deadline", rec.deadline)
	}
}

type publicationRecorder struct {
	reports int
	closed  int
	last    *evalspb.Run
	err     error

	contextKey   any
	contextValue any
	contextErr   error
	deadline     time.Time
}

func (r *publicationRecorder) ReportRun(ctx context.Context, run *evalspb.Run) error {
	r.reports++
	r.last = run
	if r.contextKey != nil {
		r.contextValue = ctx.Value(r.contextKey)
	}
	r.contextErr = ctx.Err()
	r.deadline, _ = ctx.Deadline()
	return r.err
}

func (r *publicationRecorder) Close() error {
	r.closed++
	return nil
}

func stubStandardReporter(t *testing.T, r report.Reporter, err error) func() {
	t.Helper()
	old := newStandardReporter
	newStandardReporter = func(context.Context) (report.Reporter, error) {
		return r, err
	}
	return func() { newStandardReporter = old }
}
