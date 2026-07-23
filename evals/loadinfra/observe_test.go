package loadinfra

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestObserveCloudRunSuccess(t *testing.T) {
	t.Parallel()
	window := ObservationWindow{
		Start: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 16, 10, 5, 0, 0, time.UTC),
	}
	target := CloudRunTarget{
		ID: "search", Role: RoleEntry,
		ProjectID: "proj", Region: "europe-west1", ServiceName: "search-v1",
	}
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		cloudRunMetricFilter(target, crMetricRequestCount):                                            int64Series(42),
		cloudRunMetricFilter(target, crMetricRequestLatencies):                                        doubleSeries(12.5),
		cloudRunMetricFilter(target, crMetricRequestCount, `metric.labels.response_code_class="5xx"`): int64Series(0),
		cloudRunMetricFilter(target, crMetricInstanceCount):                                           doubleSeries(3),
		cloudRunMetricFilter(target, crMetricCPUUtilization):                                          doubleSeries(0.55),
		cloudRunMetricFilter(target, crMetricMemoryUtilization):                                       doubleSeries(0.4),
		cloudRunMetricFilter(target, crMetricStartupLatencies):                                        doubleSeries(800),
	}}

	got, err := Observe(context.Background(), client, []CloudRunTarget{target}, nil, window, true, 0)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if len(got.CloudRun) != 1 {
		t.Fatalf("CloudRun len=%d, want 1", len(got.CloudRun))
	}
	snap := got.CloudRun[0]
	if snap.FetchStatus != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK {
		t.Fatalf("FetchStatus=%v, want OK (message=%v)", snap.FetchStatus, snap.FetchMessage)
	}
	if snap.Metrics.RequestCount != 42 {
		t.Fatalf("RequestCount=%d, want 42", snap.Metrics.RequestCount)
	}
	if snap.Metrics.Latency == nil || snap.Metrics.Latency.P50Ms == 0 {
		t.Fatalf("Latency=%v, want populated", snap.Metrics.Latency)
	}
}

func TestObserveSpannerSuccess(t *testing.T) {
	t.Parallel()
	window := ObservationWindow{
		Start: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 16, 10, 5, 0, 0, time.UTC),
	}
	target := SpannerTarget{
		ID: "orders-db", ProjectID: "proj", InstanceID: "prod",
		Location: "europe-west1", Database: "orders",
	}
	cpuFilter := strings.Join([]string{
		spannerResourceFilter(target),
		`metric.type="spanner.googleapis.com/instance/cpu/utilization"`,
	}, " AND ")
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		spannerMetricFilter(target, spMetricQueryCount):                               int64Series(100),
		spannerMetricFilter(target, spMetricQueryCount, `metric.labels.status!="ok"`): int64Series(2),
		spannerMetricFilter(target, spMetricAPILatencies):                             doubleSeries(5.2),
		spannerMetricFilter(target, spMetricQueryLatencies):                           doubleSeries(7.1),
		cpuFilter: doubleSeries(0.72),
	}}

	got, err := Observe(context.Background(), client, nil, []SpannerTarget{target}, window, true, 0)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if len(got.Spanner) != 1 {
		t.Fatalf("Spanner len=%d, want 1", len(got.Spanner))
	}
	snap := got.Spanner[0]
	if snap.Role != evalspb.InfraTargetRole_INFRA_TARGET_ROLE_DEPENDENCY {
		t.Fatalf("Role=%v, want DEPENDENCY", snap.Role)
	}
	if snap.FetchStatus != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK {
		t.Fatalf("FetchStatus=%v message=%v", snap.FetchStatus, snap.FetchMessage)
	}
	if snap.Metrics.QueryCount != 100 || snap.Metrics.QueryErrorCount != 2 {
		t.Fatalf("QueryCount=%d QueryErrorCount=%d", snap.Metrics.QueryCount, snap.Metrics.QueryErrorCount)
	}
}

func TestObservePartialFailure(t *testing.T) {
	t.Parallel()
	window := ObservationWindow{
		Start: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 16, 10, 5, 0, 0, time.UTC),
	}
	target := CloudRunTarget{
		ID: "api", Role: RoleEntry,
		ProjectID: "proj", Region: "europe-west1", ServiceName: "api",
	}
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		cloudRunMetricFilter(target, crMetricRequestCount): int64Series(10),
	}}

	got, err := Observe(context.Background(), client, []CloudRunTarget{target}, nil, window, true, 0)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	snap := got.CloudRun[0]
	if snap.FetchStatus != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK {
		t.Fatalf("FetchStatus=%v, want OK on partial success", snap.FetchStatus)
	}
	if snap.FetchMessage == nil || !strings.Contains(*snap.FetchMessage, "partial metric failures") {
		t.Fatalf("FetchMessage=%v, want partial failure note", snap.FetchMessage)
	}
	if snap.Metrics.RequestCount != 10 {
		t.Fatalf("RequestCount=%d, want 10", snap.Metrics.RequestCount)
	}
}

func TestObserveAllTargetsEmittedOnFailure(t *testing.T) {
	t.Parallel()
	window := ObservationWindow{
		Start: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 16, 10, 1, 0, 0, time.UTC),
	}
	cloud := CloudRunTarget{ID: "cr", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "s"}
	spanner := SpannerTarget{ID: "sp", ProjectID: "p", InstanceID: "i", Location: "r", Database: "d"}
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{}}

	got, err := Observe(context.Background(), client, []CloudRunTarget{cloud}, []SpannerTarget{spanner}, window, true, 0)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if len(got.CloudRun) != 1 || len(got.Spanner) != 1 {
		t.Fatalf("snapshots cloud=%d spanner=%d, want 1 each", len(got.CloudRun), len(got.Spanner))
	}
	if got.CloudRun[0].FetchStatus != evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE {
		t.Fatalf("cloud status=%v", got.CloudRun[0].FetchStatus)
	}
	if got.CloudRun[0].Metrics.RequestCount != 0 {
		t.Fatalf("cloud RequestCount=%d, want 0", got.CloudRun[0].Metrics.RequestCount)
	}
	if got.Spanner[0].Metrics.QueryCount != 0 {
		t.Fatalf("spanner QueryCount=%d, want 0", got.Spanner[0].Metrics.QueryCount)
	}
}

func TestObserveExtendQueryEnd(t *testing.T) {
	t.Parallel()
	window := ObservationWindow{
		Start: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 16, 10, 5, 0, 0, time.UTC),
	}
	target := CloudRunTarget{ID: "cr", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "s"}
	filter := cloudRunMetricFilter(target, crMetricRequestCount)

	t.Run("standalone", func(t *testing.T) {
		t.Parallel()
		client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
			filter: int64Series(1),
		}}
		if _, err := Observe(context.Background(), client, []CloudRunTarget{target}, nil, window, false, 0); err != nil {
			t.Fatalf("Observe: %v", err)
		}
		if !client.LastIntervalEnd.Equal(window.End) {
			t.Fatalf("query end=%v, want reported end=%v", client.LastIntervalEnd, window.End)
		}
	})

	t.Run("load integrated", func(t *testing.T) {
		t.Parallel()
		client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
			filter: int64Series(1),
		}}
		if _, err := Observe(context.Background(), client, []CloudRunTarget{target}, nil, window, true, 0); err != nil {
			t.Fatalf("Observe: %v", err)
		}
		wantEnd := window.End.Add(CloudRunSettlePadding)
		if !client.LastIntervalEnd.Equal(wantEnd) {
			t.Fatalf("query end=%v, want extended end=%v", client.LastIntervalEnd, wantEnd)
		}
	})
}

func TestObserveShortWindowAdvisory(t *testing.T) {
	t.Parallel()
	window := ObservationWindow{
		Start: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 16, 10, 0, 30, 0, time.UTC),
	}
	target := CloudRunTarget{ID: "cr", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "s"}
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		cloudRunMetricFilter(target, crMetricRequestCount): int64Series(1),
	}}

	got, err := Observe(context.Background(), client, []CloudRunTarget{target}, nil, window, true, 0)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if got.CloudRun[0].FetchMessage == nil || !strings.Contains(*got.CloudRun[0].FetchMessage, "coarse_window") {
		t.Fatalf("FetchMessage=%v, want coarse_window advisory", got.CloudRun[0].FetchMessage)
	}
}

func TestObserve_respectsTargetConcurrencyBound(t *testing.T) {
	t.Parallel()

	const (
		targets = 20
		bound   = 4
	)
	window := ObservationWindow{
		Start: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 16, 10, 5, 0, 0, time.UTC),
	}
	client := &FakeMetricClient{
		BlockDelay: 20 * time.Millisecond,
		ByFilter:   map[string][]*monitoringpb.TimeSeries{},
	}
	cloud := make([]CloudRunTarget, targets)
	for i := range cloud {
		cloud[i] = CloudRunTarget{
			ID: fmt.Sprintf("cr-%d", i), Role: RoleEntry,
			ProjectID: "p", Region: "r", ServiceName: fmt.Sprintf("svc-%d", i),
		}
		client.ByFilter[cloudRunMetricFilter(cloud[i], crMetricRequestCount)] = int64Series(1)
	}

	if _, err := Observe(context.Background(), client, cloud, nil, window, true, bound); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if client.PeakInFlight > bound {
		t.Fatalf("PeakInFlight=%d, want <= %d", client.PeakInFlight, bound)
	}
}

func int64Series(v int64) []*monitoringpb.TimeSeries {
	return []*monitoringpb.TimeSeries{{
		Resource: &monitoredres.MonitoredResource{Type: "cloud_run_revision"},
		Metric:   &metric.Metric{Type: "test"},
		Points:   []*monitoringpb.Point{{Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{Int64Value: v}}, Interval: &monitoringpb.TimeInterval{EndTime: timestamppb.Now()}}},
	}}
}

func doubleSeries(v float64) []*monitoringpb.TimeSeries {
	return []*monitoringpb.TimeSeries{{
		Resource: &monitoredres.MonitoredResource{Type: "cloud_run_revision"},
		Metric:   &metric.Metric{Type: "test"},
		Points:   []*monitoringpb.Point{{Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: v}}, Interval: &monitoringpb.TimeInterval{EndTime: timestamppb.Now()}}},
	}}
}
