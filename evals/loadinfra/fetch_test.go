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

func TestObserve_spannerWithNoFailedQueriesIsCleanOK(t *testing.T) {
	t.Parallel()
	target := SpannerTarget{ID: "orders-db", ProjectID: "p", InstanceID: "prod", Location: "europe-west1", Database: "orders"}
	cpuFilter := strings.Join(
		[]string{spannerResourceFilter(target), `metric.type="spanner.googleapis.com/instance/cpu/utilization"`},
		" AND ",
	)
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		spannerMetricFilter(target, spMetricQueryCount):     int64Series(100),
		spannerMetricFilter(target, spMetricAPILatencies):   doubleSeries(0.005),
		spannerMetricFilter(target, spMetricQueryLatencies): doubleSeries(0.007),
		cpuFilter: doubleSeries(0.72),
	}}

	got, err := Observe(
		context.Background(),
		Request{Client: client, Targets: Targets{Spanner: []SpannerTarget{target}}, Window: fetchTestWindow},
	)
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	snap := got.Spanner[0]
	if snap.GetFetchStatus() != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK {
		t.Fatalf("FetchStatus = %v, want OK", snap.GetFetchStatus())
	}
	if snap.GetMetrics().GetQueryErrorCount() != 0 {
		t.Fatalf("QueryErrorCount = %d, want 0", snap.GetMetrics().GetQueryErrorCount())
	}
	if snap.FetchMessage != nil {
		t.Fatalf("FetchMessage = %q, want nil", snap.GetFetchMessage())
	}
}

func TestObserve_cloudRunReusesRequestCountForErrorRate(t *testing.T) {
	t.Parallel()
	target := fetchTestCloudRun
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		cloudRunMetricFilter(target, crMetricRequestCount):      int64Series(40),
		cloudRunMetricFilter(target, crMetricRequestLatencies):  doubleSeries(12.5),
		cloudRunMetricFilter(target, crMetricInstanceCount):     doubleSeries(3),
		cloudRunMetricFilter(target, crMetricCPUUtilization):    doubleSeries(0.55),
		cloudRunMetricFilter(target, crMetricMemoryUtilization): doubleSeries(0.4),
		cloudRunMetricFilter(target, crMetricStartupLatencies):  doubleSeries(800),
	}}

	got, err := Observe(
		context.Background(),
		Request{Client: client, Targets: Targets{CloudRun: []CloudRunTarget{target}}, Window: fetchTestWindow},
	)
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	// request_count 1, latency 3, 5xx subset 1, instances 1, cpu 1, memory 1, startup 1.
	if client.Calls != 9 {
		t.Fatalf("Monitoring calls = %d, want 9", client.Calls)
	}
	snap := got.CloudRun[0]
	if snap.GetMetrics().Error_5XxRate == nil || snap.GetMetrics().GetError_5XxRate() != 0 {
		t.Fatalf("Error_5XxRate = %v, want 0", snap.GetMetrics().Error_5XxRate)
	}
	if snap.FetchMessage != nil {
		t.Fatalf("FetchMessage = %q, want nil", snap.GetFetchMessage())
	}
}

func TestObserve_cloudRunSkipsErrorRateWithoutRequestCount(t *testing.T) {
	t.Parallel()
	target := fetchTestCloudRun
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		cloudRunMetricFilter(target, crMetricInstanceCount): doubleSeries(1),
	}}

	got, err := Observe(
		context.Background(),
		Request{Client: client, Targets: Targets{CloudRun: []CloudRunTarget{target}}, Window: fetchTestWindow},
	)
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	snap := got.CloudRun[0]
	if snap.GetMetrics().Error_5XxRate != nil {
		t.Fatalf("Error_5XxRate = %v, want unset", snap.GetMetrics().GetError_5XxRate())
	}
	if strings.Contains(snap.GetFetchMessage(), "error_5xx_rate") {
		t.Fatalf("FetchMessage = %q, want no error_5xx_rate entry", snap.GetFetchMessage())
	}
	if !strings.Contains(snap.GetFetchMessage(), cloudRunMetricFilter(target, crMetricRequestCount)+": no data") {
		t.Fatalf("FetchMessage = %q, want request_count no data entry", snap.GetFetchMessage())
	}
}

func TestObserve_namesMissingLatencyPercentiles(t *testing.T) {
	t.Parallel()
	target := fetchTestCloudRun
	latency := cloudRunMetricFilter(target, crMetricRequestLatencies)
	count := cloudRunMetricFilter(target, crMetricRequestCount)
	client := &FakeMetricClient{
		Handler: func(_ context.Context, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
			switch {
			case req.Filter == latency && req.GetAggregation().GetCrossSeriesReducer() == monitoringpb.Aggregation_REDUCE_PERCENTILE_95:
				return nil, status.Error(codes.Unavailable, "backend unavailable")
			case req.Filter == latency:
				return doubleSeries(12.5), nil
			case req.Filter == count:
				return int64Series(10), nil
			}
			return nil, nil
		},
	}

	got, err := Observe(
		context.Background(),
		Request{Client: client, Targets: Targets{CloudRun: []CloudRunTarget{target}}, Window: fetchTestWindow},
	)
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	snap := got.CloudRun[0]
	if snap.GetFetchStatus() != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK {
		t.Fatalf("FetchStatus = %v, want OK", snap.GetFetchStatus())
	}
	if want := latency + ": missing percentiles p95"; !strings.Contains(snap.GetFetchMessage(), want) {
		t.Fatalf("FetchMessage = %q, want it to contain %q", snap.GetFetchMessage(), want)
	}
	if snap.GetMetrics().GetLatency().GetP50Ms() != 12.5 || snap.GetMetrics().GetLatency().GetP99Ms() != 12.5 {
		t.Fatalf("Latency = %v, want p50 and p99 12.5", snap.GetMetrics().GetLatency())
	}
}
