package evals

import (
	"context"
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/env"
)

func TestNewIntegrationSuite_errorsOnEmptyName(t *testing.T) {
	t.Parallel()
	if _, err := NewIntegrationSuite(""); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestNewIntegrationSuite_errorsOnDottedName(t *testing.T) {
	t.Parallel()
	if _, err := NewIntegrationSuite("a.b"); err == nil {
		t.Fatal("expected error for name containing '.'")
	}
}

func TestSuite_Case_errorsOnDuplicate(t *testing.T) {
	t.Parallel()
	s, err := NewIntegrationSuite("dup-" + t.Name())
	if err != nil {
		t.Fatalf("NewIntegrationSuite: %v", err)
	}
	if err := s.Case("one", func(context.Context, *T) {}); err != nil {
		t.Fatalf("first Case: %v", err)
	}
	if err := s.Case("one", func(context.Context, *T) {}); err == nil {
		t.Fatal("expected error for duplicate case")
	}
}

func TestSuite_Case_errorsOnNilFunc(t *testing.T) {
	t.Parallel()
	s, err := NewIntegrationSuite("nil-" + t.Name())
	if err != nil {
		t.Fatalf("NewIntegrationSuite: %v", err)
	}
	if err := s.Case("bad", nil); err == nil {
		t.Fatal("expected error for nil case func")
	}
}

func TestSuite_MustCase_panicsOnNilFunc(t *testing.T) {
	t.Parallel()
	s := MustNewIntegrationSuite("must-nil-" + t.Name())
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil case func")
		}
	}()
	s.MustCase("bad", nil)
}

func TestSuite_declarationOrder(t *testing.T) {
	t.Parallel()
	var order []string
	s := MustNewIntegrationSuite("order-" + t.Name())
	s.MustCase("first", func(_ context.Context, tr *T) {
		order = append(order, "first")
		tr.Check("ok", true)
	}).MustCase("second", func(_ context.Context, tr *T) {
		order = append(order, "second")
		tr.Check("ok", true)
	}).MustCase("third", func(_ context.Context, tr *T) {
		order = append(order, "third")
		tr.Check("ok", true)
	})

	cases := s.test.Cases()
	if len(cases) != 3 {
		t.Fatalf("cases = %d, want 3", len(cases))
	}
	for _, c := range cases {
		c.Run(context.Background())
	}
	want := []string{"first", "second", "third"}
	for i, w := range want {
		if order[i] != w {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], w)
		}
	}
}

func TestSuite_TestCase_assemblesChecks(t *testing.T) {
	t.Parallel()
	s := MustNewIntegrationSuite("assemble-" + t.Name())
	s.MustCase("mixed", func(_ context.Context, tr *T) {
		tr.Check("ok", true)
		tr.Check("bad", false)
	})
	result := s.test.Cases()[0].Run(context.Background())
	if len(result.Checks) != 2 {
		t.Fatalf("checks = %d, want 2", len(result.Checks))
	}
	if result.Status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", result.Status)
	}
}

func TestSuite_EvalCase_assemblesMetrics(t *testing.T) {
	t.Parallel()
	s := MustNewAgentEvalSuite("eval-assemble-" + t.Name())
	s.MustCase("scored", func(_ context.Context, tr *T) {
		tr.Score("quality", 0.9, 0.5, "great")
	})
	result := s.eval.Cases()[0].Run(context.Background())
	if len(result.Metrics) != 1 {
		t.Fatalf("metrics = %d, want 1", len(result.Metrics))
	}
	if result.Metrics[0].Score == nil || *result.Metrics[0].Score != 0.9 {
		t.Fatalf("score = %v", result.Metrics[0].Score)
	}
	if result.Status != evalspb.Status_PASSED {
		t.Fatalf("status = %v, want PASSED", result.Status)
	}
}

func TestSuite_UnknownEnvironmentErrors(t *testing.T) {
	t.Parallel()
	if _, err := NewIntegrationSuite("env-"+t.Name(), WithEnv("missing-"+t.Name())); err == nil {
		t.Fatal("expected error for unknown environment")
	}
}

func TestSuite_WithEnvSucceeds(t *testing.T) {
	t.Parallel()
	name := "env-known-" + t.Name()
	if err := env.Register(name); err != nil {
		t.Fatalf("env.Register: %v", err)
	}
	s, err := NewIntegrationSuite("uses-env-"+t.Name(), WithEnv(name))
	if err != nil {
		t.Fatalf("NewIntegrationSuite: %v", err)
	}
	if got := s.test.Environments(); len(got) != 1 || got[0] != name {
		t.Fatalf("environments = %v", got)
	}
}

func TestRegisterIntegration_errorsOnEvalSuite(t *testing.T) {
	t.Parallel()
	if err := RegisterIntegration(MustNewAgentEvalSuite("wrong-kind-" + t.Name())); err == nil {
		t.Fatal("expected error when registering eval suite as integration")
	}
}

func TestRegisterEval_errorsOnTestSuite(t *testing.T) {
	t.Parallel()
	if err := RegisterEval(MustNewIntegrationSuite("wrong-kind-" + t.Name())); err == nil {
		t.Fatal("expected error when registering test suite as eval")
	}
}
