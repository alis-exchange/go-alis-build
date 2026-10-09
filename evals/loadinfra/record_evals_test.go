package loadinfra_test

import (
	"context"
	"testing"

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
