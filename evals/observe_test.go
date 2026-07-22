package evals

import (
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/registry"
	"go.alis.build/evals/suite"
)

func TestNewInfraObserveSuite_WithLookbackAndTargets(t *testing.T) {
	t.Parallel()
	s, err := NewInfraObserveSuite("peak",
		WithLookback(30*time.Minute),
		WithCloudRunTargets(CloudRunTarget{
			ID: "entry", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
		}),
	)
	if err != nil {
		t.Fatalf("NewInfraObserveSuite: %v", err)
	}
	if s.inner.Lookback() != 30*time.Minute {
		t.Fatalf("lookback=%v", s.inner.Lookback())
	}
}

func TestInfraObserveCase_emptyName(t *testing.T) {
	t.Parallel()
	s := MustNewInfraObserveSuite("peak", WithLookback(time.Minute),
		WithCloudRunTargets(CloudRunTarget{
			ID: "entry", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
		}),
	)
	err := s.InfraObserveCase("")
	var invalid suite.ErrInvalidCaseName
	if !errors.As(err, &invalid) {
		t.Fatalf("InfraObserveCase() error = %v, want ErrInvalidCaseName", err)
	}
}

func TestRegisterInfraObserve_selectsRuns(t *testing.T) {
	t.Parallel()
	reg := registry.New()
	s := MustNewInfraObserveSuite("peak", WithLookback(time.Minute),
		WithCloudRunTargets(CloudRunTarget{
			ID: "entry", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "svc",
		}),
	)
	s.MustInfraObserveCase("hourly")
	if err := reg.RegisterInfraObserveSuite(s.inner); err != nil {
		t.Fatalf("RegisterInfraObserveSuite: %v", err)
	}

	runs, err := reg.SelectInfraObserveRuns([]string{"peak.hourly"})
	if err != nil {
		t.Fatalf("SelectInfraObserveRuns: %v", err)
	}
	if len(runs) != 1 || len(runs[0].Cases) != 1 {
		t.Fatalf("runs=%+v", runs)
	}
	if err := reg.ValidateSelection(evalspb.Run_INFRA_OBSERVATION, []string{"peak.hourly"}); err != nil {
		t.Fatalf("ValidateSelection: %v", err)
	}
}

func TestRegisterInfraObserve_nilSuite(t *testing.T) {
	t.Parallel()
	if err := RegisterInfraObserve(nil); err == nil {
		t.Fatal("expected error for nil suite")
	}
}

func TestNewInfraObserveSuite_requiresTargets(t *testing.T) {
	t.Parallel()
	_, err := NewInfraObserveSuite("peak", WithLookback(time.Minute))
	if err == nil {
		t.Fatal("expected error for zero targets")
	}
}
