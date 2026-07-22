package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/loadinfra"
	"go.alis.build/evals/suite"
)

func testCloudRunEntryTarget() loadinfra.CloudRunTarget {
	return loadinfra.CloudRunTarget{
		ID: "entry", Role: loadinfra.RoleEntry,
		ProjectID: "p", Region: "r", ServiceName: "svc",
	}
}

func loadSuiteWithInfra(cases ...suite.LoadCase) []suite.LoadSuiteRun {
	return []suite.LoadSuiteRun{{
		Name:     "suite-a",
		CloudRun: []loadinfra.CloudRunTarget{testCloudRunEntryTarget()},
		Cases:    cases,
	}}
}

func TestRunLoadSuites_cancelMidSuiteClosesConstructedMetricClient(t *testing.T) {
	t.Parallel()

	fake := &loadinfra.FakeMetricClient{}
	ctx, cancel := context.WithCancel(context.Background())
	runs := loadSuiteWithInfra(
		stubLoadCase{name: "one", result: passedLoad("one")},
		cancelOnRunLoadCase{name: "two", cancel: cancel},
		stubLoadCase{name: "three", result: passedLoad("three")},
	)

	_, err := New(WithMetricClientFactory(func(context.Context) (loadinfra.MetricClient, error) {
		return fake, nil
	})).RunLoadSuites(ctx, runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunLoadSuites() error = %v, want context.Canceled", err)
	}
	if fake.CloseCalls != 1 {
		t.Fatalf("CloseCalls = %d, want 1 on cancel mid-suite", fake.CloseCalls)
	}
}

func TestRunLoadSuites_eachSuiteClosesConstructedMetricClient(t *testing.T) {
	t.Parallel()

	fake := &loadinfra.FakeMetricClient{}
	runs := []suite.LoadSuiteRun{
		{
			Name:     "suite-1",
			CloudRun: []loadinfra.CloudRunTarget{testCloudRunEntryTarget()},
			Cases:    []suite.LoadCase{stubLoadCase{name: "a", result: passedLoad("a")}},
		},
		{
			Name:     "suite-2",
			CloudRun: []loadinfra.CloudRunTarget{testCloudRunEntryTarget()},
			Cases:    []suite.LoadCase{stubLoadCase{name: "b", result: passedLoad("b")}},
		},
	}

	_, err := New(WithMetricClientFactory(func(context.Context) (loadinfra.MetricClient, error) {
		return fake, nil
	})).RunLoadSuites(context.Background(), runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), nil, nil)
	if err != nil {
		t.Fatalf("RunLoadSuites() error = %v", err)
	}
	if fake.CloseCalls != 2 {
		t.Fatalf("CloseCalls = %d, want 2 (one per suite)", fake.CloseCalls)
	}
}

func TestRunLoadSuites_callerSuppliedMetricClientNotClosed(t *testing.T) {
	t.Parallel()

	fake := &loadinfra.FakeMetricClient{}
	ctx := loadinfra.WithClient(context.Background(), fake)
	runs := loadSuiteWithInfra(stubLoadCase{name: "one", result: passedLoad("one")})

	_, err := New().RunLoadSuites(ctx, runs, evalspb.RunLoadTestRequest_MINIMAL, defaultResolver(), nil, nil)
	if err != nil {
		t.Fatalf("RunLoadSuites() error = %v", err)
	}
	if fake.CloseCalls != 0 {
		t.Fatalf("CloseCalls = %d, want 0 (caller-owned client)", fake.CloseCalls)
	}
}

func TestRunInfraObserveSuites_eachSuiteClosesConstructedMetricClient(t *testing.T) {
	t.Parallel()

	fake := &loadinfra.FakeMetricClient{}
	cloud := testCloudRunEntryTarget()
	var hits int32
	runs := []suite.InfraObserveSuiteRun{
		{
			Name:     "peak-1",
			Lookback: time.Minute,
			CloudRun: []loadinfra.CloudRunTarget{cloud},
			Cases:    []suite.InfraObserveCase{slowInfraObserveCase{name: "a", hits: &hits}},
		},
		{
			Name:     "peak-2",
			Lookback: time.Minute,
			CloudRun: []loadinfra.CloudRunTarget{cloud},
			Cases:    []suite.InfraObserveCase{slowInfraObserveCase{name: "b", hits: &hits}},
		},
	}

	_, err := New(WithMetricClientFactory(func(context.Context) (loadinfra.MetricClient, error) {
		return fake, nil
	})).RunInfraObserveSuites(context.Background(), runs, InfraObserveRunParams{}, nil, nil)
	if err != nil {
		t.Fatalf("RunInfraObserveSuites() error = %v", err)
	}
	if fake.CloseCalls != 2 {
		t.Fatalf("CloseCalls = %d, want 2 (one per suite)", fake.CloseCalls)
	}
}

func TestRunInfraObserveSuites_callerSuppliedMetricClientNotClosed(t *testing.T) {
	t.Parallel()

	fake := &loadinfra.FakeMetricClient{}
	ctx := loadinfra.WithClient(context.Background(), fake)
	cloud := testCloudRunEntryTarget()
	var hits int32
	runs := []suite.InfraObserveSuiteRun{{
		Name:     "peak",
		Lookback: time.Minute,
		CloudRun: []loadinfra.CloudRunTarget{cloud},
		Cases:    []suite.InfraObserveCase{slowInfraObserveCase{name: "a", hits: &hits}},
	}}

	_, err := New().RunInfraObserveSuites(ctx, runs, InfraObserveRunParams{}, nil, nil)
	if err != nil {
		t.Fatalf("RunInfraObserveSuites() error = %v", err)
	}
	if fake.CloseCalls != 0 {
		t.Fatalf("CloseCalls = %d, want 0 (caller-owned client)", fake.CloseCalls)
	}
}
