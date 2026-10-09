package loadinfra

import (
	"context"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var fetchTestWindow = ObservationWindow{
	Start: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
	End:   time.Date(2026, 7, 16, 10, 5, 0, 0, time.UTC),
}

var fetchTestCloudRun = CloudRunTarget{ID: "api", Role: RoleEntry, ProjectID: "p", Region: "europe-west1", ServiceName: "api"}

func TestObserve_reportsPermissionDeniedWhenAnyMetricIsDenied(t *testing.T) {
	t.Parallel()
	latency := cloudRunMetricFilter(fetchTestCloudRun, crMetricRequestLatencies)
	client := &FakeMetricClient{
		Handler: func(_ context.Context, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
			if req.Filter == latency {
				return nil, status.Error(codes.PermissionDenied, "monitoring.timeSeries.list denied")
			}
			return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
		},
	}

	got, err := Observe(
		context.Background(),
		Request{Client: client, Targets: Targets{CloudRun: []CloudRunTarget{fetchTestCloudRun}}, Window: fetchTestWindow},
	)
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	snap := got.CloudRun[0]
	if snap.GetFetchStatus() != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_PERMISSION_DENIED {
		t.Fatalf("FetchStatus = %v, want PERMISSION_DENIED (message=%q)", snap.GetFetchStatus(), snap.GetFetchMessage())
	}
	if !strings.Contains(snap.GetFetchMessage(), "; ") {
		t.Fatalf("FetchMessage = %q, want ; separated metric errors", snap.GetFetchMessage())
	}
}

func TestObserve_reportsTimeoutWhenEveryMetricTimesOut(t *testing.T) {
	t.Parallel()
	client := &FakeMetricClient{Err: status.Error(codes.DeadlineExceeded, "deadline exceeded")}

	got, err := Observe(
		context.Background(),
		Request{Client: client, Targets: Targets{CloudRun: []CloudRunTarget{fetchTestCloudRun}}, Window: fetchTestWindow},
	)
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	if got.CloudRun[0].GetFetchStatus() != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_TIMEOUT {
		t.Fatalf("FetchStatus = %v, want TIMEOUT", got.CloudRun[0].GetFetchStatus())
	}
}
