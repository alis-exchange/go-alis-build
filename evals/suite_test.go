package evals

import (
	"context"
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/env"
)

func TestNewSuite_panicsOnEmptyName(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for empty name")
		}
	}()
	_ = NewSuite("")
}

func TestNewSuite_panicsOnDottedName(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for name containing '.'")
		}
	}()
	_ = NewSuite("a.b")
}

func TestSuite_Case_panicsOnDuplicate(t *testing.T) {
	t.Parallel()
	s := NewSuite("dup-" + t.Name())
	s.Case("one", func(context.Context, *T) {})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for duplicate case")
		}
	}()
	s.Case("one", func(context.Context, *T) {})
}

func TestSuite_Case_panicsOnNilFunc(t *testing.T) {
	t.Parallel()
	s := NewSuite("nil-" + t.Name())
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil case func")
		}
	}()
	s.Case("bad", nil)
}

func TestSuite_declarationOrder(t *testing.T) {
	t.Parallel()
	var order []string
	s := NewSuite("order-" + t.Name())
	s.Case("first", func(_ context.Context, tr *T) {
		order = append(order, "first")
		tr.Check("ok", true)
	})
	s.Case("second", func(_ context.Context, tr *T) {
		order = append(order, "second")
		tr.Check("ok", true)
	})
	s.Case("third", func(_ context.Context, tr *T) {
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
	s := NewSuite("assemble-" + t.Name())
	s.Case("mixed", func(_ context.Context, tr *T) {
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
	s := NewEvalSuite("eval-assemble-" + t.Name())
	s.Case("scored", func(_ context.Context, tr *T) {
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

func TestSuite_UnknownEnvironmentPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for unknown environment")
		}
	}()
	_ = NewSuite("env-"+t.Name(), WithEnv("missing-"+t.Name()))
}

func TestSuite_WithEnvSucceeds(t *testing.T) {
	t.Parallel()
	name := "env-known-" + t.Name()
	env.Register(name)
	s := NewSuite("uses-env-"+t.Name(), WithEnv(name))
	if got := s.test.Environments(); len(got) != 1 || got[0] != name {
		t.Fatalf("environments = %v", got)
	}
}

func TestRegisterIntegration_panicsOnEvalSuite(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when registering eval suite as integration")
		}
	}()
	RegisterIntegration(NewEvalSuite("wrong-kind-" + t.Name()))
}

func TestRegisterEval_panicsOnTestSuite(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when registering test suite as eval")
		}
	}()
	RegisterEval(NewSuite("wrong-kind-" + t.Name()))
}
