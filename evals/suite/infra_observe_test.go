package suite

import (
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/loadinfra"
)

func TestResolveInfraObserveLookback_precedence(t *testing.T) {
	t.Parallel()
	suiteLB := 30 * time.Minute
	caseLB := 10 * time.Minute
	reqLB := 5 * time.Minute

	got, err := ResolveInfraObserveLookback(reqLB, caseLB, suiteLB, true, true)
	if err != nil || got != reqLB {
		t.Fatalf("request precedence: got=%v err=%v", got, err)
	}

	got, err = ResolveInfraObserveLookback(0, caseLB, suiteLB, false, true)
	if err != nil || got != caseLB {
		t.Fatalf("per-case precedence: got=%v err=%v", got, err)
	}

	got, err = ResolveInfraObserveLookback(0, 0, suiteLB, false, false)
	if err != nil || got != suiteLB {
		t.Fatalf("suite default: got=%v err=%v", got, err)
	}

	if _, err := ResolveInfraObserveLookback(0, 0, 0, false, false); err == nil {
		t.Fatal("expected lookback unset error")
	}
}

func TestNewInfraObserveSuite_requiresPositiveLookback(t *testing.T) {
	t.Parallel()
	if _, err := NewInfraObserveSuite("s",
		WithLookback(0),
		WithInfraObserveCloudRunTargets(validInfraObserveCloudRunTarget()),
	); err == nil {
		t.Fatal("expected error for zero lookback")
	}
}

func TestNewInfraObserveSuite_requiresTargets(t *testing.T) {
	t.Parallel()
	_, err := NewInfraObserveSuite("s", WithLookback(time.Minute))
	if err == nil {
		t.Fatal("expected error for zero targets")
	}
	if !errors.Is(err, ErrInfraObserveNoTargets{}) {
		t.Fatalf("err=%v, want ErrInfraObserveNoTargets", err)
	}
}

func TestInfraObserveSuite_AddCase_qualifiesNames(t *testing.T) {
	t.Parallel()
	s, err := NewInfraObserveSuite("peak",
		WithLookback(time.Minute),
		WithInfraObserveCloudRunTargets(validInfraObserveCloudRunTarget()),
	)
	if err != nil {
		t.Fatalf("NewInfraObserveSuite: %v", err)
	}
	c := stubInfraObserveCase{name: "hourly"}
	if err := s.AddCase(c); err != nil {
		t.Fatalf("AddCase: %v", err)
	}
	if got := s.SelectInfraObserveCases(nil)[0].Name(); got != "peak.hourly" {
		t.Fatalf("qualified name=%q", got)
	}
}

func TestQualifiedInfraObserveCase_Run_stampsName(t *testing.T) {
	t.Parallel()
	q := &qualifiedInfraObserveCase{
		name:  "peak.hourly",
		inner: stubInfraObserveCase{name: "hourly"},
	}
	got := q.Run(context.Background(), InfraObserveCaseConfig{})
	if got.Name != "peak.hourly" {
		t.Fatalf("name=%q, want peak.hourly", got.Name)
	}
}

type stubInfraObserveCase struct {
	name string
}

func (c stubInfraObserveCase) Name() string { return c.name }

func (c stubInfraObserveCase) Lookback() (time.Duration, bool) { return 0, false }

func (c stubInfraObserveCase) Run(ctx context.Context, cfg InfraObserveCaseConfig) *execution.InfraObserveCaseResult {
	return &execution.InfraObserveCaseResult{Name: c.Name(), Status: evalspb.Status_PASSED}
}

func validInfraObserveCloudRunTarget() loadinfra.CloudRunTarget {
	return loadinfra.CloudRunTarget{
		ID: "entry", Role: loadinfra.RoleEntry,
		ProjectID: "p", Region: "r", ServiceName: "svc",
	}
}
