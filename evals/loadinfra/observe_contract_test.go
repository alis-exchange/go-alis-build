package loadinfra

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/evals/loadgen"
)

func TestObserveLoad_queriesMeasurementWindowWithoutExtension(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	target := CloudRunTarget{ID: "checkout", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "checkout"}
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		cloudRunMetricFilter(target, crMetricRequestCount): int64Series(1),
	}}

	got, err := ObserveLoad(context.Background(), client, Targets{
		CloudRun: []CloudRunTarget{target},
	}, &loadgen.Metrics{MeasurementStart: start, MeasurementEnd: end})
	if err != nil {
		t.Fatalf("ObserveLoad() error = %v", err)
	}
	if got.Window.Start != start || got.Window.End != end {
		t.Fatalf("result window = %+v, want %v..%v", got.Window, start, end)
	}
	snapshot := got.CloudRun[0]
	if !snapshot.GetWindowStart().AsTime().Equal(start) || !snapshot.GetWindowEnd().AsTime().Equal(end) {
		t.Fatalf("reported window = %v..%v, want %v..%v", snapshot.GetWindowStart().AsTime(), snapshot.GetWindowEnd().AsTime(), start, end)
	}
	if !client.LastIntervalEnd.Equal(end) {
		t.Fatalf("query end = %v, want measurement end %v", client.LastIntervalEnd, end)
	}
}

func TestObserveLoad_rejectsNilMetricsWithoutQuerying(t *testing.T) {
	t.Parallel()

	client := &FakeMetricClient{}
	_, err := ObserveLoad(context.Background(), client, Targets{
		CloudRun: []CloudRunTarget{{ID: "checkout"}},
	}, nil)
	if err == nil || err.Error() != "loadinfra: nil load metrics" {
		t.Fatalf("ObserveLoad() error = %v, want nil load metrics", err)
	}
	if client.Calls != 0 {
		t.Fatalf("Monitoring calls = %d, want 0", client.Calls)
	}
}

// fixedClock returns a clock that always reports t.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestObserveLookback_settlesForTargetKinds(t *testing.T) {
	t.Parallel()

	const lookback = 30 * time.Minute
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	target := SpannerTarget{ID: "orders", ProjectID: "p", InstanceID: "i", Location: "r", Database: "d"}
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		spannerMetricFilter(target, spMetricQueryCount): int64Series(1),
	}}

	got, err := Observe(context.Background(), lookbackRequest(client, Targets{Spanner: []SpannerTarget{target}}, lookback, fixedClock(now)))
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	wantEnd := now.Add(-SpannerSettlePadding)
	wantStart := wantEnd.Add(-lookback)
	if got.Window.Start != wantStart || got.Window.End != wantEnd {
		t.Fatalf("result window = %+v, want %v..%v", got.Window, wantStart, wantEnd)
	}
	snap := got.Spanner[0]
	if !snap.GetWindowStart().AsTime().Equal(wantStart) || !snap.GetWindowEnd().AsTime().Equal(wantEnd) {
		t.Fatalf("snapshot window = %v..%v, want %v..%v", snap.GetWindowStart().AsTime(), snap.GetWindowEnd().AsTime(), wantStart, wantEnd)
	}
	if !client.LastIntervalEnd.Equal(wantEnd) {
		t.Fatalf("query end = %v, want %v", client.LastIntervalEnd, wantEnd)
	}
}

func TestObserve_returnsSnapshotsInDeterministicTargetOrder(t *testing.T) {
	t.Parallel()

	window := ObservationWindow{
		Start: time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 23, 10, 5, 0, 0, time.UTC),
	}
	cloud := []CloudRunTarget{
		{ID: "z-api", Role: RoleDependency, ProjectID: "p", Region: "r", ServiceName: "z"},
		{ID: "a-api", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "a"},
	}
	spanner := []SpannerTarget{
		{ID: "z-db", ProjectID: "p", InstanceID: "z", Location: "r", Database: "db"},
		{ID: "a-db", ProjectID: "p", InstanceID: "a", Location: "r", Database: "db"},
	}
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{}}
	for _, target := range cloud {
		client.ByFilter[cloudRunMetricFilter(target, crMetricRequestCount)] = int64Series(1)
	}
	for _, target := range spanner {
		client.ByFilter[spannerMetricFilter(target, spMetricQueryCount)] = int64Series(1)
	}

	got, err := observeForTest(context.Background(), client, cloud, spanner, window, false, 1)
	if err != nil {
		t.Fatalf("observeForTest() error = %v", err)
	}
	if got.CloudRun[0].GetId() != "a-api" || got.CloudRun[1].GetId() != "z-api" {
		t.Fatalf("cloud order = [%s %s], want [a-api z-api]", got.CloudRun[0].GetId(), got.CloudRun[1].GetId())
	}
	if got.Spanner[0].GetId() != "a-db" || got.Spanner[1].GetId() != "z-db" {
		t.Fatalf("spanner order = [%s %s], want [a-db z-db]", got.Spanner[0].GetId(), got.Spanner[1].GetId())
	}
}

func TestObserve_preservesTargetMetadataRolesAndReportedWindow(t *testing.T) {
	t.Parallel()

	window := ObservationWindow{
		Start: time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 23, 10, 5, 0, 0, time.UTC),
	}
	cloud := CloudRunTarget{
		ID: "checkout-api", Role: RoleEntry, ProjectID: "product-project",
		Region: "europe-west1", ServiceName: "checkout-v1", Revision: "checkout-v1-00042",
	}
	spanner := SpannerTarget{
		ID: "orders-db", ProjectID: "data-project", InstanceID: "orders",
		Location: "europe-west1", Database: "main",
	}
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		cloudRunMetricFilter(cloud, crMetricRequestCount): int64Series(1),
		spannerMetricFilter(spanner, spMetricQueryCount):  int64Series(1),
	}}

	got, err := observeForTest(context.Background(), client, []CloudRunTarget{cloud}, []SpannerTarget{spanner}, window, true, 0)
	if err != nil {
		t.Fatalf("observeForTest() error = %v", err)
	}
	cr := got.CloudRun[0]
	if cr.GetRole() != evalspb.InfraTargetRole_INFRA_TARGET_ROLE_ENTRY {
		t.Fatalf("cloud role = %v, want ENTRY", cr.GetRole())
	}
	if cr.GetTarget().GetProjectId() != "product-project" ||
		cr.GetTarget().GetRegion() != "europe-west1" ||
		cr.GetTarget().GetServiceName() != "checkout-v1" ||
		cr.GetTarget().GetRevision() != "checkout-v1-00042" {
		t.Fatalf("cloud target metadata = %+v", cr.GetTarget())
	}
	if !cr.GetWindowStart().AsTime().Equal(window.Start) || !cr.GetWindowEnd().AsTime().Equal(window.End) {
		t.Fatalf(
			"cloud reported window = %v..%v, want %v..%v",
			cr.GetWindowStart().AsTime(),
			cr.GetWindowEnd().AsTime(),
			window.Start,
			window.End,
		)
	}

	sp := got.Spanner[0]
	if sp.GetRole() != evalspb.InfraTargetRole_INFRA_TARGET_ROLE_DEPENDENCY {
		t.Fatalf("spanner role = %v, want DEPENDENCY", sp.GetRole())
	}
	if sp.GetTarget().GetProjectId() != "data-project" ||
		sp.GetTarget().GetInstanceId() != "orders" ||
		sp.GetTarget().GetLocation() != "europe-west1" ||
		sp.GetTarget().GetDatabase() != "main" {
		t.Fatalf("spanner target metadata = %+v", sp.GetTarget())
	}
	if !sp.GetWindowStart().AsTime().Equal(window.Start) || !sp.GetWindowEnd().AsTime().Equal(window.End) {
		t.Fatalf(
			"spanner reported window = %v..%v, want %v..%v",
			sp.GetWindowStart().AsTime(),
			sp.GetWindowEnd().AsTime(),
			window.Start,
			window.End,
		)
	}
}

func TestObserve_standaloneQueriesUseSettledWindowAsReported(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	targets := Targets{CloudRun: []CloudRunTarget{{}}}
	window := lookbackWindow(30*time.Minute, now, targets)
	target := CloudRunTarget{ID: "checkout-api", Role: RoleEntry, ProjectID: "p", Region: "r", ServiceName: "checkout"}
	client := &FakeMetricClient{ByFilter: map[string][]*monitoringpb.TimeSeries{
		cloudRunMetricFilter(target, crMetricRequestCount): int64Series(1),
	}}

	got, err := observeForTest(context.Background(), client, []CloudRunTarget{target}, nil, window, false, 0)
	if err != nil {
		t.Fatalf("observeForTest() error = %v", err)
	}
	if want := queryWindow(window).End; !client.LastIntervalEnd.Equal(want) {
		t.Fatalf("query end = %v, want rounded settled end %v", client.LastIntervalEnd, want)
	}
	if !got.CloudRun[0].GetWindowEnd().AsTime().Equal(window.End) {
		t.Fatalf("snapshot window_end = %v, want %v", got.CloudRun[0].GetWindowEnd().AsTime(), window.End)
	}
}
