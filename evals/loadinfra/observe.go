package loadinfra

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/evals/loadgen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// perTargetTimeout caps each Monitoring ListTimeSeries call so one slow
	// target cannot block Observe indefinitely when many targets run concurrently.
	perTargetTimeout = 30 * time.Second
	// DefaultTargetConcurrency bounds concurrent target fetches inside Observe.
	DefaultTargetConcurrency = 8
)

// ObserveResult holds infra snapshots for one observation window.
type ObserveResult struct {
	// Window is the reported interval shared by every returned snapshot.
	Window ObservationWindow
	// CloudRun holds one snapshot per declared Cloud Run target, including
	// targets with zero request_count.
	CloudRun []*evalspb.CloudRunTargetSnapshot
	// Spanner holds one snapshot per declared Spanner target, including targets
	// with zero query_count.
	Spanner []*evalspb.SpannerTargetSnapshot
}

// Request describes one custom-window infrastructure observation.
type Request struct {
	// Client queries Cloud Monitoring.
	Client MetricClient
	// Targets declares the services and databases to observe.
	Targets Targets
	// Window is the interval reported on returned snapshots.
	Window ObservationWindow
	// ExtendQueryEnd adds per-kind ingestion padding to Monitoring queries
	// without changing the reported window.
	ExtendQueryEnd bool
	// TargetConcurrency bounds simultaneous target observations. Values below
	// one use DefaultTargetConcurrency.
	TargetConcurrency int
}

// ObserveLoad observes the measurement window from a generated load run.
// Monitoring query ends are extended by each target kind's settle padding,
// while snapshots retain the original measurement window. Nil metrics return
// an error without querying Monitoring.
func ObserveLoad(ctx context.Context, client MetricClient, targets Targets, metrics *loadgen.Metrics) (ObserveResult, error) {
	if metrics == nil {
		return ObserveResult{}, fmt.Errorf("loadinfra: nil load metrics")
	}
	return Observe(ctx, Request{
		Client:         client,
		Targets:        targets,
		Window:         windowFromMetrics(metrics),
		ExtendQueryEnd: true,
	})
}

// ObserveLookback observes a settled window ending before Monitoring's
// visibility delay for the declared target kinds.
func ObserveLookback(ctx context.Context, client MetricClient, targets Targets, lookback time.Duration) (ObserveResult, error) {
	return Observe(ctx, Request{
		Client:  client,
		Targets: targets,
		Window:  lookbackWindow(lookback, time.Now().UTC(), targets),
	})
}

// Observe fetches snapshots for a caller-defined reported window. A positive
// TargetConcurrency bounds concurrent target fetches; zero or negative uses
// [DefaultTargetConcurrency].
func Observe(ctx context.Context, req Request) (ObserveResult, error) {
	if req.Client == nil {
		return ObserveResult{}, fmt.Errorf("loadinfra: nil MetricClient")
	}
	if len(req.Targets.CloudRun) == 0 && len(req.Targets.Spanner) == 0 {
		return ObserveResult{Window: req.Window}, nil
	}

	limit := targetConcurrencyLimit(req.TargetConcurrency)
	sem := make(chan struct{}, limit)

	var (
		mu  sync.Mutex
		out = ObserveResult{Window: req.Window}
		wg  sync.WaitGroup
	)

	for _, target := range req.Targets.CloudRun {
		wg.Add(1)
		go func(t CloudRunTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			snap := observeCloudRun(ctx, req.Client, t, req.Window, req.ExtendQueryEnd)
			mu.Lock()
			out.CloudRun = append(out.CloudRun, snap)
			mu.Unlock()
		}(target)
	}
	for _, target := range req.Targets.Spanner {
		wg.Add(1)
		go func(t SpannerTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			snap := observeSpanner(ctx, req.Client, t, req.Window, req.ExtendQueryEnd)
			mu.Lock()
			out.Spanner = append(out.Spanner, snap)
			mu.Unlock()
		}(target)
	}
	wg.Wait()

	sort.Slice(out.CloudRun, func(i, j int) bool {
		return out.CloudRun[i].Id < out.CloudRun[j].Id
	})
	sort.Slice(out.Spanner, func(i, j int) bool {
		return out.Spanner[i].Id < out.Spanner[j].Id
	})
	return out, nil
}

func targetConcurrencyLimit(n int) int {
	if n <= 0 {
		return DefaultTargetConcurrency
	}
	return n
}

// observeCloudRun fetches one Cloud Run target snapshot. Query failures are
// recorded on FetchStatus/FetchMessage; the snapshot is always returned.
func observeCloudRun(ctx context.Context, client MetricClient, t CloudRunTarget, w ObservationWindow, extendQueryEnd bool) *evalspb.CloudRunTargetSnapshot {
	ctx, cancel := context.WithTimeout(ctx, perTargetTimeout)
	defer cancel()

	qw := cloudRunQueryWindow(w, extendQueryEnd)
	snap := &evalspb.CloudRunTargetSnapshot{
		Id:          t.ID,
		Role:        mapTargetRole(t.Role),
		Target:      &evalspb.CloudRunTargetRef{ProjectId: t.ProjectID, Region: t.Region, ServiceName: t.ServiceName},
		WindowStart: timestamppb.New(w.Start),
		WindowEnd:   timestamppb.New(w.End),
		FetchStatus: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE,
		Metrics:     &evalspb.CloudRunMetrics{},
	}
	if t.Revision != "" {
		snap.Target.Revision = &t.Revision
	}
	if advisory := shortWindowAdvisory(w); advisory != "" {
		snap.FetchMessage = &advisory
	}

	metrics, partial, err := fetchCloudRunMetrics(ctx, client, t, qw)
	if err != nil {
		msg := err.Error()
		if snap.FetchMessage != nil {
			msg = *snap.FetchMessage + "; " + msg
		}
		snap.FetchMessage = &msg
		snap.FetchStatus = classifyFetchStatus(err)
		return snap
	}
	snap.Metrics = metrics
	if len(partial) == 0 {
		snap.FetchStatus = evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK
		return snap
	}
	snap.FetchStatus = evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK
	msg := "partial metric failures: " + strings.Join(partial, ", ")
	if snap.FetchMessage != nil {
		msg = *snap.FetchMessage + "; " + msg
	}
	snap.FetchMessage = &msg
	return snap
}

// observeSpanner fetches one Spanner target snapshot. Spanner role is always
// DEPENDENCY on the wire regardless of suite declaration.
func observeSpanner(ctx context.Context, client MetricClient, t SpannerTarget, w ObservationWindow, extendQueryEnd bool) *evalspb.SpannerTargetSnapshot {
	ctx, cancel := context.WithTimeout(ctx, perTargetTimeout)
	defer cancel()

	qw := spannerQueryWindow(w, extendQueryEnd)
	snap := &evalspb.SpannerTargetSnapshot{
		Id:          t.ID,
		Role:        evalspb.InfraTargetRole_INFRA_TARGET_ROLE_DEPENDENCY,
		Target:      &evalspb.SpannerTargetRef{ProjectId: t.ProjectID, InstanceId: t.InstanceID, Location: t.Location, Database: t.Database},
		WindowStart: timestamppb.New(w.Start),
		WindowEnd:   timestamppb.New(w.End),
		FetchStatus: evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE,
		Metrics:     &evalspb.SpannerMetrics{},
	}
	if advisory := shortWindowAdvisory(w); advisory != "" {
		snap.FetchMessage = &advisory
	}

	metrics, partial, err := fetchSpannerMetrics(ctx, client, t, qw)
	if err != nil {
		msg := err.Error()
		if snap.FetchMessage != nil {
			msg = *snap.FetchMessage + "; " + msg
		}
		snap.FetchMessage = &msg
		snap.FetchStatus = classifyFetchStatus(err)
		return snap
	}
	snap.Metrics = metrics
	if len(partial) == 0 {
		snap.FetchStatus = evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK
		return snap
	}
	snap.FetchStatus = evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_OK
	msg := "partial metric failures: " + strings.Join(partial, ", ")
	if snap.FetchMessage != nil {
		msg = *snap.FetchMessage + "; " + msg
	}
	snap.FetchMessage = &msg
	return snap
}

// mapTargetRole converts suite TargetRole to the wire InfraTargetRole enum.
func mapTargetRole(r TargetRole) evalspb.InfraTargetRole {
	switch r {
	case RoleEntry:
		return evalspb.InfraTargetRole_INFRA_TARGET_ROLE_ENTRY
	default:
		return evalspb.InfraTargetRole_INFRA_TARGET_ROLE_DEPENDENCY
	}
}

// shortWindowAdvisory returns a wire-visible hint when the observation window
// is shorter than the Monitoring alignment period.
func shortWindowAdvisory(w ObservationWindow) string {
	// Sub-minute windows use 60s Monitoring alignment; percentiles and rates may be coarse.
	if w.End.Sub(w.Start) < 60*time.Second {
		return "coarse_window"
	}
	return ""
}

// metricOutcome records one metric fetch attempt for mergeOutcomes.
type metricOutcome struct {
	// ok is true when this metric contributed a value to the snapshot.
	ok bool
	// partial lists per-metric gap messages from nested partial failures.
	partial []string
	// err is non-nil when this metric query failed entirely.
	err error
}

// mergeOutcomes combines per-metric fetch outcomes. At least one metric must
// succeed; otherwise the target snapshot is marked unavailable. Partial gaps
// are listed in the returned slice and appended to FetchMessage.
func mergeOutcomes(outcomes ...metricOutcome) ([]string, error) {
	var partial []string
	var succeeded int
	for _, o := range outcomes {
		if o.err != nil {
			partial = append(partial, o.err.Error())
			continue
		}
		if o.ok {
			succeeded++
		}
		partial = append(partial, o.partial...)
	}
	if succeeded == 0 {
		if len(partial) == 0 {
			return nil, fmt.Errorf("no metrics returned")
		}
		return nil, fmt.Errorf("%s", strings.Join(partial, "; "))
	}
	return partial, nil
}

// querySeries issues one ListTimeSeries request with optional aggregation.
func querySeries(ctx context.Context, client MetricClient, projectID string, window ObservationWindow, filter string, agg *monitoringpb.Aggregation) ([]*monitoringpb.TimeSeries, error) {
	req := &monitoringpb.ListTimeSeriesRequest{
		Filter: filter,
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(window.Start),
			EndTime:   timestamppb.New(window.End),
		},
	}
	if agg != nil {
		req.Aggregation = agg
	}
	return client.QueryTimeSeries(ctx, projectID, req)
}
