package evals

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/loadgen"
	"go.alis.build/evals/suite"
)

func TestLoadCase_negativeLatencySLO(t *testing.T) {
	t.Parallel()

	s := MustNewLoadSuite("s-" + t.Name())
	err := s.LoadCase("c",
		TransportTarget(func(context.Context) error { return nil }),
		[]SLO{SLOLatencyP99(-time.Second)},
	)
	var invalid ErrInvalidSLO
	if !errors.As(err, &invalid) {
		t.Fatalf("LoadCase() error = %v, want ErrInvalidSLO", err)
	}
}

func TestLoadCase_negativeErrorRateSLO(t *testing.T) {
	t.Parallel()

	s := MustNewLoadSuite("s-" + t.Name())
	err := s.LoadCase("c",
		TransportTarget(func(context.Context) error { return nil }),
		[]SLO{SLOErrorRate(-0.5)},
	)
	var invalid ErrInvalidSLO
	if !errors.As(err, &invalid) {
		t.Fatalf("LoadCase() error = %v, want ErrInvalidSLO", err)
	}
}

func TestNewLoadSuite_nanProfileQPS(t *testing.T) {
	t.Parallel()

	_, err := NewLoadSuite("s-"+t.Name(), WithLoadProfile(evalspb.RunLoadTestRequest_MODERATE, loadgen.Profile{
		QPS:         math.NaN(),
		Concurrency: 2,
		Duration:    time.Second,
	}))
	var invalid loadgen.ErrInvalidProfile
	if !errors.As(err, &invalid) {
		t.Fatalf("NewLoadSuite() error = %v, want ErrInvalidProfile", err)
	}
	if invalid.Field != "QPS" {
		t.Fatalf("Field = %q, want QPS", invalid.Field)
	}
}

func TestNewLoadSuite_nilOption(t *testing.T) {
	t.Parallel()

	var nilOpt LoadSuiteOption
	_, err := NewLoadSuite("s-"+t.Name(), nilOpt)
	var nilOption suite.ErrNilOption
	if !errors.As(err, &nilOption) {
		t.Fatalf("NewLoadSuite() error = %v, want ErrNilOption", err)
	}
}

func TestLoadCase_nilOption(t *testing.T) {
	t.Parallel()

	s := MustNewLoadSuite("s-" + t.Name())
	var nilOpt LoadCaseOption
	err := s.LoadCase("c",
		TransportTarget(func(context.Context) error { return nil }),
		NoSLOs(),
		nilOpt,
	)
	var nilOption suite.ErrNilOption
	if !errors.As(err, &nilOption) {
		t.Fatalf("LoadCase() error = %v, want ErrNilOption", err)
	}
}

func TestNewIntegrationSuite_nilOption(t *testing.T) {
	t.Parallel()

	var nilOpt SuiteOption
	_, err := NewIntegrationSuite("s-"+t.Name(), nilOpt)
	var nilOption suite.ErrNilOption
	if !errors.As(err, &nilOption) {
		t.Fatalf("NewIntegrationSuite() error = %v, want ErrNilOption", err)
	}
}
