package loadinfra

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// perTargetTimeout caps each Monitoring ListTimeSeries call so one slow
	// target cannot block Observe indefinitely when many targets run concurrently.
	perTargetTimeout = 30 * time.Second
)

// ObserveResult holds infra snapshots for one observation window.
type ObserveResult struct {
	// CloudRun holds one snapshot per declared Cloud Run target, including
	// targets with zero request_count.
	CloudRun []*evalspb.CloudRunTargetSnapshot
	// Spanner holds one snapshot per declared Spanner target, including targets
	// with zero query_count.
	Spanner []*evalspb.SpannerTargetSnapshot
}

// Observe fetches Cloud Run and Spanner snapshots for all declared targets over
// the reported window w. Load-integrated callers pass WindowFromMetrics with
// extendQueryEnd true so Monitoring queries extend w.End forward by per-kind
// settle padding. Standalone callers pass WindowLookback with extendQueryEnd
// false so queries use the settled window as-is.
func Observe(ctx context.Context, client MetricClient, cloud []CloudRunTarget, spanner []SpannerTarget, w ObservationWindow, extendQueryEnd bool) (ObserveResult, error) {
	if client == nil {
		return ObserveResult{}, fmt.Errorf("loadinfra: nil MetricClient")
	}
	if len(cloud) == 0 && len(spanner) == 0 {
		return ObserveResult{}, nil
	}

	var (
		mu  sync.Mutex
		out ObserveResult
		wg  sync.WaitGroup
	)

	for _, target := range cloud {
		wg.Add(1)
		go func(t CloudRunTarget) {
			defer wg.Done()
			snap := observeCloudRun(ctx, client, t, w, extendQueryEnd)
			mu.Lock()
			out.CloudRun = append(out.CloudRun, snap)
			mu.Unlock()
		}(target)
	}
	for _, target := range spanner {
		wg.Add(1)
		go func(t SpannerTarget) {
			defer wg.Done()
			snap := observeSpanner(ctx, client, t, w, extendQueryEnd)
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

// observeCloudRun fetches one Cloud Run target snapshot. Query failures are
// recorded on FetchStatus/FetchMessage; the snapshot is always returned.
func observeCloudRun(ctx context.Context, client MetricClient, t CloudRunTarget, w ObservationWindow, extendQueryEnd bool) *evalspb.CloudRunTargetSnapshot {
	ctx, cancel := context.WithTimeout(ctx, perTargetTimeout)
	defer cancel()

	qw := cloudRunQueryWindow(w, extendQueryEnd)
	snap := &evalspb.CloudRunTargetSnapshot{
		Id:           t.ID,
		Role:         mapTargetRole(t.Role),
		Target:       &evalspb.CloudRunTargetRef{ProjectId: t.ProjectID, Region: t.Region, ServiceName: t.ServiceName},
		WindowStart:  timestamppb.New(w.Start),
		WindowEnd:    timestamppb.New(w.End),
		FetchStatus:  evalspb.InfraFetchStatus_INFRA_FETCH_STATUS_UNAVAILABLE,
		Metrics:      &evalspb.CloudRunMetrics{},
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
