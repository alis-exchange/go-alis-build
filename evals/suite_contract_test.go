package evals

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/validation"
)

func TestConstructors_requireStableSuiteNames(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name      string
		construct func(string) runnableSuite
	}{
		{name: "integration", construct: func(n string) runnableSuite { return NewIntegrationSuite(n) }},
		{name: "agent", construct: func(n string) runnableSuite { return NewAgentEvalSuite(n) }},
		{name: "load", construct: func(n string) runnableSuite { return NewLoadSuite(n) }},
		{name: "infra", construct: func(n string) runnableSuite { return NewInfraObservationSuite(n) }},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			suite := tc.construct("stable-" + t.Name())
			suite.add("only", noopIntegrationCase)
			if _, err := suite.run(context.Background()); err != nil {
				t.Fatalf("Run() with valid name error = %v, want nil", err)
			}

			bad := tc.construct("")
			bad.add("only", noopIntegrationCase)
			_, err := bad.run(context.Background())
			var cfg *ConfigErrors
			if !errors.As(err, &cfg) {
				t.Fatalf("Run() error = %v, want ConfigErrors for empty suite name", err)
			}
			if !errors.Is(err, ErrEmptySuiteName{}) {
				t.Fatalf("ConfigErrors = %v, want ErrEmptySuiteName", err)
			}
		})
	}
}

func TestAddCase_returnsSameSuiteAndPreservesOrder(t *testing.T) {
	t.Parallel()

	s := NewIntegrationSuite("order-"+t.Name()).
		AddCase("first", noopIntegrationCase).
		AddCase("second", noopIntegrationCase)

	if s.core.cases[0].name != "first" || s.core.cases[1].name != "second" {
		t.Fatalf("case order = [%s, %s], want [first, second]", s.core.cases[0].name, s.core.cases[1].name)
	}
}

func TestRun_aggregateConfigErrors(t *testing.T) {
	t.Parallel()

	s := NewIntegrationSuite("cfg-"+t.Name()).
		AddCase("", noopIntegrationCase).
		AddCase("dup", noopIntegrationCase).
		AddCase("dup", noopIntegrationCase).
		AddCase("nil-fn", nil)

	_, err := s.Run(context.Background(),
		WithMaxConcurrency(0),
		WithReporter(nil),
		nil,
	)
	var cfg *ConfigErrors
	if !errors.As(err, &cfg) {
		t.Fatalf("Run() error = %v, want ConfigErrors", err)
	}
	for _, want := range []error{
		ErrInvalidCaseName{},
		ErrDuplicateCase{Case: "dup"},
		ErrNilCaseFunc{Case: "nil-fn"},
		ErrInvalidConcurrency{Value: 0},
		ErrNilReporter{},
		ErrNilOption{},
	} {
		if !errors.Is(err, want) {
			t.Errorf("ConfigErrors missing %T: %v", want, err)
		}
	}
}

func TestSuite_sealingReuseAndLateAddCase(t *testing.T) {
	t.Parallel()

	s := NewIntegrationSuite("seal-"+t.Name()).
		AddCase("only", noopIntegrationCase)

	var wg sync.WaitGroup
	const runners = 4
	errs := make([]error, runners)
	for i := 0; i < runners; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = s.Run(context.Background())
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("Run[%d] error = %v, want nil", i, err)
		}
	}

	s.AddCase("late", noopIntegrationCase)
	_, err := s.Run(context.Background())
	if !errors.Is(err, ErrSuiteSealed) {
		t.Fatalf("Run() after late AddCase error = %v, want ErrSuiteSealed", err)
	}
}

func TestRun_metadataPrecedenceAndSilentReporter(t *testing.T) {
	t.Setenv("ALIS_OS_PROJECT", "from-env")
	rec := &recordingReporter{}

	s := NewIntegrationSuite("meta-"+t.Name()).
		AddCase("case-a", noopIntegrationCase)

	run, err := s.Run(context.Background(),
		WithBatchID("batch-1"),
		WithOperation("operations/op-1"),
		WithGoogleProjectID("override-project"),
		WithReporter(rec),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if rec.calls != 0 {
		t.Fatalf("Run invoked reporter %d times, want 0", rec.calls)
	}
	assertRunEnvelope(t, run, evalspb.Run_INTEGRATION_TEST, runEnvelope{
		batchID:       "batch-1",
		operation:     "operations/op-1",
		googleProject: "override-project",
		caseCount:     1,
	})

	_, err = s.RunAndPublish(context.Background(), WithReporter(rec))
	if err != nil {
		t.Fatalf("RunAndPublish() error = %v", err)
	}
	if rec.calls != 1 {
		t.Fatalf("RunAndPublish reporter calls = %d, want 1", rec.calls)
	}
}

func TestRun_branchIdentity(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		build    func() (*evalspb.Run, error)
		wantType evalspb.Run_Type
	}{
		{
			name: "integration",
			build: func() (*evalspb.Run, error) {
				return NewIntegrationSuite("branch-int").AddCase("c", noopIntegrationCase).Run(context.Background())
			},
			wantType: evalspb.Run_INTEGRATION_TEST,
		},
		{
			name: "agent",
			build: func() (*evalspb.Run, error) {
				return NewAgentEvalSuite("branch-agent").AddCase("c", noopAgentCase).Run(context.Background())
			},
			wantType: evalspb.Run_AGENT_EVAL,
		},
		{
			name: "load",
			build: func() (*evalspb.Run, error) {
				return NewLoadSuite("branch-load").AddCase("c", noopLoadCase).Run(context.Background())
			},
			wantType: evalspb.Run_LOAD_TEST,
		},
		{
			name: "infra",
			build: func() (*evalspb.Run, error) {
				return NewInfraObservationSuite("branch-infra").AddCase("c", noopObservationCase).Run(context.Background())
			},
			wantType: evalspb.Run_INFRA_OBSERVATION,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			run, err := tc.build()
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if run.GetType() != tc.wantType {
				t.Fatalf("Run.type = %v, want %v", run.GetType(), tc.wantType)
			}
			switch tc.wantType {
			case evalspb.Run_INTEGRATION_TEST:
				if run.GetIntegrationTest() == nil {
					t.Fatal("integration_test branch missing")
				}
			case evalspb.Run_AGENT_EVAL:
				if run.GetAgentEval() == nil {
					t.Fatal("agent_eval branch missing")
				}
			case evalspb.Run_LOAD_TEST:
				if run.GetLoadTest() == nil {
					t.Fatal("load_test branch missing")
				}
			case evalspb.Run_INFRA_OBSERVATION:
				if run.GetInfraObservation() == nil {
					t.Fatal("infra_observation branch missing")
				}
			}
		})
	}
}

func TestRun_defaultConcurrencyIsOne(t *testing.T) {
	t.Parallel()

	var active atomic.Int32
	var peak atomic.Int32
	caseFn := func(context.Context, *LoadResult) {
		current := active.Add(1)
		for {
			observed := peak.Load()
			if current <= observed || peak.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
	}
	s := NewLoadSuite("conc-"+t.Name()).
		AddCase("a", caseFn).
		AddCase("b", caseFn)

	_, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := peak.Load(); got != 1 {
		t.Fatalf("peak active cases = %d, want 1", got)
	}
}

func TestRun_caseFailureIsResultData(t *testing.T) {
	t.Parallel()

	s := NewIntegrationSuite("failure-"+t.Name()).
		AddCase("broken", func(_ context.Context, v *validation.Validator) {
			v.Custom("must pass", false)
		})

	run, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil for an evaluated case failure", err)
	}
	if run.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("Run.status = %v, want FAILED", run.GetStatus())
	}
}

func TestRunAndPublish_allSuiteTypes(t *testing.T) {
	t.Parallel()

	rec := &recordingReporter{}
	suites := []func() (*evalspb.Run, error){
		func() (*evalspb.Run, error) {
			return NewIntegrationSuite("publish-int").AddCase("case", noopIntegrationCase).
				RunAndPublish(context.Background(), WithReporter(rec))
		},
		func() (*evalspb.Run, error) {
			return NewAgentEvalSuite("publish-agent").AddCase("case", noopAgentCase).
				RunAndPublish(context.Background(), WithReporter(rec))
		},
		func() (*evalspb.Run, error) {
			return NewLoadSuite("publish-load").AddCase("case", noopLoadCase).
				RunAndPublish(context.Background(), WithReporter(rec))
		},
		func() (*evalspb.Run, error) {
			return NewInfraObservationSuite("publish-infra").AddCase("case", noopObservationCase).
				RunAndPublish(context.Background(), WithReporter(rec))
		},
	}
	for i, run := range suites {
		if _, err := run(); err != nil {
			t.Fatalf("RunAndPublish[%d]() error = %v", i, err)
		}
	}
	if rec.calls != len(suites) {
		t.Fatalf("reporter calls = %d, want %d", rec.calls, len(suites))
	}
}

type runEnvelope struct {
	batchID       string
	operation     string
	googleProject string
	caseCount     int
}

type recordingReporter struct {
	calls int
}

func (r *recordingReporter) ReportRun(context.Context, *evalspb.Run) error {
	r.calls++
	return nil
}

func assertRunEnvelope(t *testing.T, run *evalspb.Run, wantType evalspb.Run_Type, want runEnvelope) {
	t.Helper()
	if run == nil {
		t.Fatal("run is nil")
	}
	if !strings.HasPrefix(run.GetName(), "runs/") {
		t.Fatalf("run.name = %q, want runs/{uuid}", run.GetName())
	}
	if run.GetType() != wantType {
		t.Fatalf("run.type = %v, want %v", run.GetType(), wantType)
	}
	if run.GetStartTime() == nil || run.GetEndTime() == nil || run.GetCreateTime() == nil {
		t.Fatalf("timestamps missing: start=%v end=%v create=%v", run.GetStartTime(), run.GetEndTime(), run.GetCreateTime())
	}
	if run.GetOperation() != want.operation {
		t.Fatalf("operation = %q, want %q", run.GetOperation(), want.operation)
	}
	if run.GetGoogleProjectId() != want.googleProject {
		t.Fatalf("google_project_id = %q, want %q", run.GetGoogleProjectId(), want.googleProject)
	}
	if want.batchID != "" {
		if run.GetBatchId() != want.batchID {
			t.Fatalf("batch_id = %q, want %q", run.GetBatchId(), want.batchID)
		}
	}
}

type runnableSuite interface {
	add(name string, fn IntegrationCaseFunc)
	run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error)
}

func (s *IntegrationSuite) add(name string, fn IntegrationCaseFunc) { s.AddCase(name, fn) }
func (s *IntegrationSuite) run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.Run(ctx, opts...)
}

func (s *AgentEvalSuite) add(name string, fn IntegrationCaseFunc) {
	s.AddCase(name, func(ctx context.Context, _ *AgentEvalResult) { fn(ctx, validation.NewValidator()) })
}
func (s *AgentEvalSuite) run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.Run(ctx, opts...)
}

func (s *LoadSuite) add(name string, fn IntegrationCaseFunc) {
	s.AddCase(name, func(ctx context.Context, _ *LoadResult) { fn(ctx, validation.NewValidator()) })
}
func (s *LoadSuite) run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.Run(ctx, opts...)
}

func (s *InfraObservationSuite) add(name string, fn IntegrationCaseFunc) {
	s.AddCase(name, func(ctx context.Context, _ *InfraObservationResult) { fn(ctx, validation.NewValidator()) })
}
func (s *InfraObservationSuite) run(ctx context.Context, opts ...RunOption) (*evalspb.Run, error) {
	return s.Run(ctx, opts...)
}

func noopIntegrationCase(context.Context, *validation.Validator) {}

func noopAgentCase(context.Context, *AgentEvalResult) {}

func noopLoadCase(context.Context, *LoadResult) {}

func noopObservationCase(context.Context, *InfraObservationResult) {}
