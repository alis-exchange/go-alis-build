package loadinfra

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func observeForTest(
	ctx context.Context,
	client MetricClient,
	cloud []CloudRunTarget,
	spanner []SpannerTarget,
	window ObservationWindow,
	extend bool,
	concurrency int,
) (ObserveResult, error) {
	return Observe(ctx, Request{
		Client:            client,
		Targets:           Targets{CloudRun: cloud, Spanner: spanner},
		Window:            window,
		ExtendQueryEnd:    extend,
		TargetConcurrency: concurrency,
	})
}

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

	got, err := observeForTest(context.Background(), client, []CloudRunTarget{target}, nil, window, true, 0)
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

	got, err := observeForTest(context.Background(), client, nil, []SpannerTarget{target}, window, true, 0)
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

	got, err := observeForTest(context.Background(), client, []CloudRunTarget{target}, nil, window, true, 0)
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

	got, err := observeForTest(context.Background(), client, []CloudRunTarget{cloud}, []SpannerTarget{spanner}, window, true, 0)
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

func TestObserve_queriesWindowRoundedOutToWholeMinutes(t *testing.T) {
	t.Parallel()
	// Deliberately not on whole minutes, so rounding the result or snapshot
	// window instead of only the query interval is caught.
	window := ObservationWindow{
		Start: time.Date(2026, 7, 16, 10, 0, 20, 0, time.UTC),
		End:   time.Date(2026, 7, 16, 10, 5, 40, 0, time.UTC),
	}
	wantStart := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 7, 16, 10, 6, 0, 0, time.UTC)
	target := CloudRunTarget{ID: "cr", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "s"}
	for _, extend := range []bool{false, true} {
		client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
			cloudRunMetricFilter(target, crMetricRequestCount): int64Series(1),
		}}
		got, err := Observe(context.Background(), Request{
			Client: client, Targets: Targets{CloudRun: []CloudRunTarget{target}},
			Window: window, ExtendQueryEnd: extend,
			now: fixedClock(time.Date(2026, 7, 16, 11, 0, 0, 0, time.UTC)),
		})
		if err != nil {
			t.Fatalf("Observe(extend=%v) error = %v", extend, err)
		}
		if got.Window != window {
			t.Fatalf("extend=%v result window = %+v, want unrounded measurement window %+v", extend, got.Window, window)
		}
		if len(client.Requests) == 0 {
			t.Fatalf("extend=%v recorded no Monitoring requests", extend)
		}
		for _, req := range client.Requests {
			if s, e := req.GetInterval().
				GetStartTime().
				AsTime(),
				req.GetInterval().
					GetEndTime().
					AsTime(); !s.Equal(wantStart) ||
				!e.Equal(wantEnd) {
				t.Fatalf("extend=%v %s interval = %v..%v, want %v..%v", extend, req.GetFilter(), s, e, wantStart, wantEnd)
			}
		}
		snap := got.CloudRun[0]
		if !snap.GetWindowStart().AsTime().Equal(window.Start) || !snap.GetWindowEnd().AsTime().Equal(window.End) {
			t.Fatalf(
				"snapshot window = %v..%v, want measurement window %v..%v",
				snap.GetWindowStart().AsTime(),
				snap.GetWindowEnd().AsTime(),
				window.Start,
				window.End,
			)
		}
	}
}

func TestObserve_addsSettleAdvisoryBeforeSettleTime(t *testing.T) {
	t.Parallel()
	window := ObservationWindow{
		Start: time.Date(2026, 7, 16, 10, 0, 20, 0, time.UTC),
		End:   time.Date(2026, 7, 16, 10, 5, 40, 0, time.UTC),
	}
	cloud := CloudRunTarget{ID: "cr", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "s"}
	spanner := SpannerTarget{ID: "sp", ProjectID: "p", InstanceID: "i", Location: "r", Database: "d"}
	tests := []struct {
		name        string
		now         time.Time
		wantAdvised bool
	}{
		{"right after the window", time.Date(2026, 7, 16, 10, 6, 0, 0, time.UTC), true},
		{"well after settle", time.Date(2026, 7, 16, 10, 20, 0, 0, time.UTC), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
				cloudRunMetricFilter(cloud, crMetricRequestCount): int64Series(1),
				spannerMetricFilter(spanner, spMetricQueryCount):  int64Series(1),
			}}
			got, err := Observe(context.Background(), Request{
				Client: client, Targets: Targets{CloudRun: []CloudRunTarget{cloud}, Spanner: []SpannerTarget{spanner}},
				Window: window, now: fixedClock(tt.now),
			})
			if err != nil {
				t.Fatalf("Observe() error = %v", err)
			}
			for _, msg := range []string{got.CloudRun[0].GetFetchMessage(), got.Spanner[0].GetFetchMessage()} {
				if got := strings.Contains(msg, settleAdvisory); got != tt.wantAdvised {
					t.Fatalf("FetchMessage = %q, advisory present = %v, want %v", msg, got, tt.wantAdvised)
				}
			}
		})
	}
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

	got, err := observeForTest(context.Background(), client, []CloudRunTarget{target}, nil, window, true, 0)
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

	if _, err := observeForTest(context.Background(), client, cloud, nil, window, true, bound); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if client.PeakInFlight > bound {
		t.Fatalf("PeakInFlight=%d, want <= %d", client.PeakInFlight, bound)
	}
}

func int64Series(v int64) []*monitoringpb.TimeSeries {
	return []*monitoringpb.TimeSeries{
		{
			Resource: &monitoredres.MonitoredResource{Type: "cloud_run_revision"},
			Metric:   &metric.Metric{Type: "test"},
			Points: []*monitoringpb.Point{
				{
					Value:    &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{Int64Value: v}},
					Interval: &monitoringpb.TimeInterval{EndTime: timestamppb.Now()},
				},
			},
		},
	}
}

func doubleSeries(v float64) []*monitoringpb.TimeSeries {
	return []*monitoringpb.TimeSeries{
		{
			Resource: &monitoredres.MonitoredResource{Type: "cloud_run_revision"},
			Metric:   &metric.Metric{Type: "test"},
			Points: []*monitoringpb.Point{
				{
					Value:    &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: v}},
					Interval: &monitoringpb.TimeInterval{EndTime: timestamppb.Now()},
				},
			},
		},
	}
}
