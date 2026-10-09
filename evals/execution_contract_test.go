package evals

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/validation"
)

func TestRun_withMaxConcurrencyBoundsAllSuiteTypes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		build func(func())
	}{
		{
			name: "integration",
			build: func(work func()) {
				NewIntegrationSuite("p4-conc-integration").
					AddCase("a", func(context.Context, *validation.Validator) { work() }).
					AddCase("b", func(context.Context, *validation.Validator) { work() }).
					AddCase("c", func(context.Context, *validation.Validator) { work() }).
					Run(context.Background(), WithMaxConcurrency(2))
			},
		},
		{
			name: "agent",
			build: func(work func()) {
				NewAgentEvalSuite("p4-conc-agent").
					AddCase("a", func(context.Context, *AgentEvalResult) { work() }).
					AddCase("b", func(context.Context, *AgentEvalResult) { work() }).
					AddCase("c", func(context.Context, *AgentEvalResult) { work() }).
					Run(context.Background(), WithMaxConcurrency(2))
			},
		},
		{
			name: "load",
			build: func(work func()) {
				NewLoadSuite("p4-conc-load").
					AddCase("a", func(context.Context, *LoadResult) { work() }).
					AddCase("b", func(context.Context, *LoadResult) { work() }).
					AddCase("c", func(context.Context, *LoadResult) { work() }).
					Run(context.Background(), WithMaxConcurrency(2))
			},
		},
		{
			name: "infra",
			build: func(work func()) {
				NewInfraObservationSuite("p4-conc-infra").
					AddCase("a", func(context.Context, *InfraObservationResult) { work() }).
					AddCase("b", func(context.Context, *InfraObservationResult) { work() }).
					AddCase("c", func(context.Context, *InfraObservationResult) { work() }).
					Run(context.Background(), WithMaxConcurrency(2))
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var active atomic.Int32
			var peak atomic.Int32
			tc.build(func() {
				current := active.Add(1)
				for {
					observed := peak.Load()
					if current <= observed || peak.CompareAndSwap(observed, current) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				active.Add(-1)
			})
			if got := peak.Load(); got > 2 {
				t.Fatalf("peak active cases = %d, want <= 2", got)
			}
			if got := peak.Load(); got < 2 {
				t.Fatalf("peak active cases = %d, want concurrency override to allow 2", got)
			}
		})
	}
}

func TestRun_preservesRegistrationOrderUnderReverseCompletion(t *testing.T) {
	t.Parallel()

	run, err := NewLoadSuite("p4-order").
		AddCase("slow", func(context.Context, *LoadResult) {
			time.Sleep(20 * time.Millisecond)
		}).
		AddCase("fast", noopLoadCase).
		Run(context.Background(), WithMaxConcurrency(2))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	cases := run.GetLoadTest().GetCases()
	if got, want := []string{cases[0].GetId(), cases[1].GetId()}, []string{"p4-order.slow", "p4-order.fast"}; got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("case order = %v, want %v", got, want)
	}
}

func TestRun_recoversPanicsForSpecializedCases(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		run  func() (*evalspb.Run, error)
		get  func(*evalspb.Run) []*evalspb.Validation
	}{
		{
			name: "agent",
			run: func() (*evalspb.Run, error) {
				return NewAgentEvalSuite("p4-panic-agent").
					AddCase("panic", func(context.Context, *AgentEvalResult) { panic("agent boom") }).
					Run(context.Background())
			},
			get: func(run *evalspb.Run) []*evalspb.Validation {
				return run.GetAgentEval().GetCases()[0].GetValidations()
			},
		},
		{
			name: "load",
			run: func() (*evalspb.Run, error) {
				return NewLoadSuite("p4-panic-load").
					AddCase("panic", func(context.Context, *LoadResult) { panic("load boom") }).
					Run(context.Background())
			},
			get: func(run *evalspb.Run) []*evalspb.Validation {
				return run.GetLoadTest().GetCases()[0].GetValidations()
			},
		},
		{
			name: "infra",
			run: func() (*evalspb.Run, error) {
				return NewInfraObservationSuite("p4-panic-infra").
					AddCase("panic", func(context.Context, *InfraObservationResult) { panic("infra boom") }).
					Run(context.Background())
			},
			get: func(run *evalspb.Run) []*evalspb.Validation {
				return run.GetInfraObservation().GetCases()[0].GetValidations()
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			run, err := tc.run()
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if run.GetStatus() != evalspb.Status_FAILED {
				t.Fatalf("run status = %v, want FAILED", run.GetStatus())
			}
			validations := tc.get(run)
			if len(validations) != 1 {
				t.Fatalf("validations = %d, want 1", len(validations))
			}
			if validations[0].GetId() != "_evals.panic" {
				t.Fatalf("panic validation id = %q, want _evals.panic", validations[0].GetId())
			}
			if validations[0].GetStatus() != evalspb.Status_FAILED {
				t.Fatalf("panic validation status = %v, want FAILED", validations[0].GetStatus())
			}
			if !strings.Contains(validations[0].GetMessage(), "boom") {
				t.Fatalf("panic validation message = %q, want panic value", validations[0].GetMessage())
			}
		})
	}
}

func TestRun_cancellationStopsNewSpecializedCasesAndReturnsPartialRun(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var started atomic.Int32
	run, err := NewAgentEvalSuite("p4-cancel-agent").
		AddCase("first", func(_ context.Context, r *AgentEvalResult) {
			started.Add(1)
			r.AddMetric(&evalspb.AgentEvalResults_Case_Metric{Id: "done", Status: evalspb.Status_PASSED})
			cancel()
		}).
		AddCase("second", func(_ context.Context, r *AgentEvalResult) {
			started.Add(1)
			r.AddMetric(&evalspb.AgentEvalResults_Case_Metric{Id: "unexpected", Status: evalspb.Status_PASSED})
		}).
		Run(ctx, WithMaxConcurrency(1))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if run == nil {
		t.Fatal("Run() returned nil run, want partial run")
	}
	if started.Load() != 1 {
		t.Fatalf("started cases = %d, want 1", started.Load())
	}
	cases := run.GetAgentEval().GetCases()
	if cases[0].GetStatus() != evalspb.Status_PASSED {
		t.Fatalf("first status = %v, want PASSED", cases[0].GetStatus())
	}
	if cases[1].GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("second status = %v, want NOT_EVALUATED", cases[1].GetStatus())
	}
	metrics := cases[1].GetMetrics()
	if len(metrics) != 1 {
		t.Fatalf("second metrics = %d, want skipped marker", len(metrics))
	}
	if metrics[0].GetId() != "_evals.skipped" ||
		metrics[0].GetStatus() != evalspb.Status_NOT_EVALUATED ||
		metrics[0].GetMessage() != "run cancelled" {
		t.Fatalf("second skipped metric = %+v, want _evals.skipped NOT_EVALUATED run cancelled", metrics[0])
	}
}

func TestLoadSuite_godocWarnsAboutParallelLoadCases(t *testing.T) {
	t.Parallel()

	doc, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("ReadFile(doc.go): %v", err)
	}
	text := string(doc)
	for _, want := range []string{"LoadSuite", "parallel load cases combine traffic", "distort measurements"} {
		if !strings.Contains(text, want) {
			t.Fatalf("doc.go missing %q", want)
		}
	}
}

func TestRun_cancellationDoesNotMarkCaseThatRan(t *testing.T) {
	t.Parallel()

	t.Run("integration", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		run, err := NewIntegrationSuite("ran-empty-integration").
			AddCase("records-nothing", func(context.Context, *validation.Validator) {
				cancel()
			}).
			AddCase("never-starts", func(_ context.Context, v *validation.Validator) {
				v.Custom("unexpected", true)
			}).
			Run(ctx, WithMaxConcurrency(1))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
		cases := run.GetIntegrationTest().GetCases()
		if len(cases) != 2 {
			t.Fatalf("case count = %d, want 2", len(cases))
		}
		if got := cases[0].GetStatus(); got != evalspb.Status_NOT_EVALUATED {
			t.Fatalf("ran case status = %v, want NOT_EVALUATED", got)
		}
		if got := len(cases[0].GetChecks()); got != 0 {
			t.Fatalf("ran case checks = %d, want 0 (no run cancelled marker)", got)
		}
		checks := cases[1].GetChecks()
		if len(checks) != 1 ||
			checks[0].GetId() != "_evals.skipped" ||
			checks[0].GetStatus() != evalspb.Status_NOT_EVALUATED ||
			checks[0].GetMessage() != "run cancelled" {
			t.Fatalf("unstarted case checks = %v, want one _evals.skipped NOT_EVALUATED run cancelled", checks)
		}
	})

	t.Run("load", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		run, err := NewLoadSuite("ran-empty-load").
			AddCase("records-nothing", func(context.Context, *LoadResult) {
				cancel()
			}).
			AddCase("never-starts", func(_ context.Context, r *LoadResult) {
				r.AddTag("unexpected", "true")
			}).
			Run(ctx, WithMaxConcurrency(1))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
		cases := run.GetLoadTest().GetCases()
		if len(cases) != 2 {
			t.Fatalf("case count = %d, want 2", len(cases))
		}
		if got := cases[0].GetStatus(); got != evalspb.Status_NOT_EVALUATED {
			t.Fatalf("ran case status = %v, want NOT_EVALUATED", got)
		}
		if got := len(cases[0].GetValidations()); got != 0 {
			t.Fatalf("ran case validations = %d, want 0 (no run cancelled marker)", got)
		}
		validations := cases[1].GetValidations()
		if len(validations) != 1 ||
			validations[0].GetId() != "_evals.skipped" ||
			validations[0].GetStatus() != evalspb.Status_NOT_EVALUATED ||
			validations[0].GetMessage() != "run cancelled" {
			t.Fatalf("unstarted case validations = %v, want one _evals.skipped NOT_EVALUATED run cancelled", validations)
		}
	})
}
