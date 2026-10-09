package loadinfra_test

import (
	"context"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/evals"
	"go.alis.build/evals/loadinfra"
)

// Both root case builders satisfy SnapshotRecorder by method set alone.
var (
	_ loadinfra.SnapshotRecorder = (*evals.LoadResult)(nil)
	_ loadinfra.SnapshotRecorder = (*evals.InfraObservationResult)(nil)
)

// TestRecordTo_loadSuite checks that RecordTo on a load case builder adds the
// observed snapshots to the case a real LoadSuite run reports.
func TestRecordTo_loadSuite(t *testing.T) {
	t.Parallel()

	obs := loadinfra.ObserveResult{
		CloudRun: []*evalspb.CloudRunTargetSnapshot{{Id: "checkout-api"}},
		Spanner:  []*evalspb.SpannerTargetSnapshot{{Id: "orders-db"}},
	}
	run, err := evals.NewLoadSuite("record-to").
		AddCase("steady", func(_ context.Context, r *evals.LoadResult) {
			obs.RecordTo(r)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetLoadTest().GetCases()[0]
	if c.GetStatus() != evalspb.Status_PASSED {
		t.Fatalf("case status = %v, want PASSED", c.GetStatus())
	}
	if got := len(c.GetCloudRun()); got != 1 || c.GetCloudRun()[0].GetId() != "checkout-api" {
		t.Fatalf("cloud_run = %v, want [checkout-api]", c.GetCloudRun())
	}
	if got := len(c.GetSpanner()); got != 1 || c.GetSpanner()[0].GetId() != "orders-db" {
		t.Fatalf("spanner = %v, want [orders-db]", c.GetSpanner())
	}
}

// observedWindow returns a hand-built result whose window is not on whole
// minutes, so a test fails if anything rounds it on the way to the case.
func observedWindow() loadinfra.ObserveResult {
	start := time.Date(2026, 10, 9, 9, 0, 20, 0, time.UTC)
	return loadinfra.ObserveResult{
		Window:   loadinfra.ObservationWindow{Start: start, End: start.Add(15*time.Minute + 20*time.Second)},
		CloudRun: []*evalspb.CloudRunTargetSnapshot{{Id: "checkout-api"}},
		Spanner:  []*evalspb.SpannerTargetSnapshot{{Id: "orders-db"}},
	}
}

// TestRecordTo_infraObservationSuite checks that RecordTo on an infra
// observation case builder sets the unrounded window and lookback and adds
// the snapshots to the case a real InfraObservationSuite run reports.
func TestRecordTo_infraObservationSuite(t *testing.T) {
	t.Parallel()

	obs := observedWindow()
	run, err := evals.NewInfraObservationSuite("record-to").
		AddCase("peak", func(_ context.Context, r *evals.InfraObservationResult) {
			obs.RecordTo(r)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetInfraObservation().GetCases()[0]
	if c.GetStatus() != evalspb.Status_PASSED {
		t.Fatalf("case status = %v, want PASSED", c.GetStatus())
	}
	if got := c.GetLookback().AsDuration(); got != 15*time.Minute+20*time.Second {
		t.Fatalf("lookback = %v, want 15m20s", got)
	}
	if got, want := c.GetWindowStart().AsTime(), time.Date(2026, 10, 9, 9, 0, 20, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("window_start = %v, want %v", got, want)
	}
	if got, want := c.GetWindowEnd().AsTime(), time.Date(2026, 10, 9, 9, 15, 40, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("window_end = %v, want %v", got, want)
	}
	if len(c.GetCloudRun()) != 1 || c.GetCloudRun()[0].GetId() != "checkout-api" {
		t.Fatalf("cloud_run = %v, want [checkout-api]", c.GetCloudRun())
	}
	if len(c.GetSpanner()) != 1 || c.GetSpanner()[0].GetId() != "orders-db" {
		t.Fatalf("spanner = %v, want [orders-db]", c.GetSpanner())
	}
}

// TestRecordTo_infraObservationSuiteRejectsSecondWindow checks that a manual
// SetWindow after RecordTo fails the case and keeps RecordTo's window.
func TestRecordTo_infraObservationSuiteRejectsSecondWindow(t *testing.T) {
	t.Parallel()

	obs := observedWindow()
	run, err := evals.NewInfraObservationSuite("record-to-twice").
		AddCase("peak", func(_ context.Context, r *evals.InfraObservationResult) {
			obs.RecordTo(r)
			r.SetWindow(30*time.Minute, obs.Window.Start, obs.Window.End)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	c := run.GetInfraObservation().GetCases()[0]
	if c.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", c.GetStatus())
	}
	if got := c.GetLookback().AsDuration(); got != 15*time.Minute+20*time.Second {
		t.Fatalf("lookback = %v, want RecordTo's 15m20s retained", got)
	}
	if len(c.GetValidations()) != 1 || c.GetValidations()[0].GetMessage() != "evals: infra observation window already set" {
		t.Fatalf("validations = %v, want one window-already-set failure", c.GetValidations())
	}
}
