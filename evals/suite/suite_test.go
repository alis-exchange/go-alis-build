package suite

import (
	"context"
	"errors"
	"testing"

	"go.alis.build/evals/execution"
)

type stubTestCase struct {
	name   string
	result *execution.CaseResult
}

func (c stubTestCase) Name() string { return c.name }

func (c stubTestCase) Run(context.Context) *execution.CaseResult {
	return c.result
}

func mustTestSuite(t *testing.T, name string, cases ...TestCase) *TestSuite {
	t.Helper()
	s, err := NewTestSuite(name)
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCases(cases...); err != nil {
		t.Fatalf("AddCases: %v", err)
	}
	return s
}

func TestNewTestSuite_emptyCaseName(t *testing.T) {
	t.Parallel()

	s, err := NewTestSuite("files-v2")
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	err = s.AddCase(stubTestCase{name: ""})
	var invalid ErrInvalidCaseName
	if !errors.As(err, &invalid) {
		t.Fatalf("AddCase() error = %v, want ErrInvalidCaseName", err)
	}
}

func TestNewTestSuite_withEnvironment(t *testing.T) {
	t.Parallel()

	s, err := NewTestSuite("files-v2", WithEnvironment("suite-test-env-"+t.Name()))
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCase(stubTestCase{name: "upload"}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	if len(s.Environments()) != 1 {
		t.Fatalf("environments = %v", s.Environments())
	}
}

func TestTestSuite_AddCase_duplicate(t *testing.T) {
	t.Parallel()

	s, err := NewTestSuite("files-v2")
	if err != nil {
		t.Fatalf("NewTestSuite: %v", err)
	}
	if err := s.AddCase(stubTestCase{name: "upload"}); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	if err := s.AddCase(stubTestCase{name: "upload"}); err == nil {
		t.Fatal("expected duplicate case error")
	}
}

func TestTestSuite_nilReceiver(t *testing.T) {
	t.Parallel()

	var s *TestSuite
	if err := s.AddCase(stubTestCase{name: "upload"}); !errors.Is(err, ErrNilSuite{}) {
		t.Fatalf("AddCase error = %v, want %v", err, ErrNilSuite{})
	}
	if err := s.AddCases(); !errors.Is(err, ErrNilSuite{}) {
		t.Fatalf("AddCases error = %v, want %v", err, ErrNilSuite{})
	}
	if s.Name() != "" || s.Cases() != nil {
		t.Fatal("nil suite accessors should return zero values")
	}
}

func TestEvalSuite_nilReceiver(t *testing.T) {
	t.Parallel()

	var s *EvalSuite
	if err := s.AddCase(stubTestCase{name: "upload"}); !errors.Is(err, ErrNilSuite{}) {
		t.Fatalf("AddCase error = %v, want %v", err, ErrNilSuite{})
	}
	if err := s.AddCases(); !errors.Is(err, ErrNilSuite{}) {
		t.Fatalf("AddCases error = %v, want %v", err, ErrNilSuite{})
	}
	if s.Name() != "" || s.Cases() != nil {
		t.Fatal("nil suite accessors should return zero values")
	}
}
