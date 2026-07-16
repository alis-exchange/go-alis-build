package mapper

import (
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
)

func TestLoadRun_mapsInfraSnapshots(t *testing.T) {
	t.Parallel()
	sr := execution.LoadSuiteResult{
		Cases: []execution.LoadCaseResult{{
			Name:   "suite.case",
			Status: evalspb.Status_PASSED,
			CloudRun: []*evalspb.CloudRunTargetSnapshot{{
				Id: "entry", FetchStatus: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK,
				Metrics: &evalspb.CloudRunMetrics{RequestCount: 12},
			}},
			Spanner: []*evalspb.SpannerTargetSnapshot{{
				Id: "db", FetchStatus: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK,
				Metrics: &evalspb.SpannerMetrics{QueryCount: 4},
			}},
		}},
	}
	run := LoadRun(sr, "op", "run-1", "")
	c := run.GetLoadTest().GetCases()[0]
	if len(c.GetCloudRun()) != 1 || c.GetCloudRun()[0].GetMetrics().GetRequestCount() != 12 {
		t.Fatalf("cloud_run=%+v", c.GetCloudRun())
	}
	if len(c.GetSpanner()) != 1 || c.GetSpanner()[0].GetMetrics().GetQueryCount() != 4 {
		t.Fatalf("spanner=%+v", c.GetSpanner())
	}
	if len(c.GetInfraChecks()) != 0 {
		t.Fatalf("infra_checks=%v, want empty in v1", c.GetInfraChecks())
	}
}

func TestInfraObserveRun_maps(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	sr := execution.InfraObserveSuiteResult{
		SuiteName: "peak",
		StartTime: start,
		EndTime:   end,
		Cases: []execution.InfraObserveCaseResult{{
			Name:        "peak.hourly",
			Status:      evalspb.Status_PASSED,
			Lookback:    30 * time.Minute,
			WindowStart: start,
			WindowEnd:   end,
			CloudRun: []*evalspb.CloudRunTargetSnapshot{{
				Id: "entry", Metrics: &evalspb.CloudRunMetrics{RequestCount: 99},
			}},
		}},
	}
	run := InfraObserveRun(sr, "operations/obs", "run-obs", "batch-1")
	if run.GetType() != evalspb.Run_INFRA_OBSERVATION {
		t.Fatalf("type=%v", run.GetType())
	}
	io := run.GetInfraObservation()
	if io == nil || len(io.GetCases()) != 1 {
		t.Fatalf("infra_observation=%+v", io)
	}
	c := io.GetCases()[0]
	if c.GetLookback().AsDuration() != 30*time.Minute {
		t.Fatalf("lookback=%v", c.GetLookback())
	}
	if c.GetCloudRun()[0].GetMetrics().GetRequestCount() != 99 {
		t.Fatalf("request_count=%d", c.GetCloudRun()[0].GetMetrics().GetRequestCount())
	}
}
