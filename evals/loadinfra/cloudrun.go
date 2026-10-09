package loadinfra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Cloud Run Monitoring metric type strings used in ListTimeSeries filters.
const (
	crMetricRequestCount      = "run.googleapis.com/request_count"
	crMetricRequestLatencies  = "run.googleapis.com/request_latencies"
	crMetricInstanceCount     = "run.googleapis.com/container/instance_count"
	crMetricCPUUtilization    = "run.googleapis.com/container/cpu/utilizations"
	crMetricMemoryUtilization = "run.googleapis.com/container/memory/utilizations"
	crMetricStartupLatencies  = "run.googleapis.com/container/startup_latencies"
)

// cloudRunResourceFilter builds the resource clause shared by all Cloud Run
// metric queries for target t.
func cloudRunResourceFilter(t CloudRunTarget) string {
	parts := []string{
		`resource.type="cloud_run_revision"`,
		fmt.Sprintf(`resource.labels.service_name="%s"`, escapeFilterLabel(t.ServiceName)),
		fmt.Sprintf(`resource.labels.location="%s"`, escapeFilterLabel(t.Region)),
	}
	if t.Revision != "" {
		parts = append(parts, fmt.Sprintf(`resource.labels.revision_name="%s"`, escapeFilterLabel(t.Revision)))
	}
	return strings.Join(parts, " AND ")
}

// cloudRunMetricFilter combines the resource filter with a metric type and
// optional extra label predicates (for example response_code_class="5xx").
func cloudRunMetricFilter(t CloudRunTarget, metricType string, extra ...string) string {
	parts := []string{cloudRunResourceFilter(t), fmt.Sprintf(`metric.type="%s"`, metricType)}
	parts = append(parts, extra...)
	return strings.Join(parts, " AND ")
}

// fetchCloudRunMetrics queries all Cloud Run metrics for one target. Returns
// partial failure messages when individual metrics are missing but at least one
// metric succeeds; returns a top-level error only when every metric fails.
func fetchCloudRunMetrics(
	ctx context.Context,
	client MetricClient,
	t CloudRunTarget,
	window ObservationWindow,
) (*evalspb.CloudRunMetrics, []string, error) {
	// Each metric is fetched independently; partial failures still emit OK snapshots
	// with FetchMessage listing missing series.
	m := &evalspb.CloudRunMetrics{}

	count, countOut := fetchSum(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricRequestCount))
	if countOut.ok {
		m.RequestCount = count
	}
	latency, latOut := fetchLatency(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricRequestLatencies))
	if latOut.ok {
		m.Latency = latency
	}
	var rateOut metricOutcome
	if countOut.ok {
		var rate float64
		rate, rateOut = error5xxRate(ctx, client, t, window, count)
		if rateOut.ok {
			m.Error_5XxRate = &rate
		}
	}
	maxInst, instOut := fetchMax(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricInstanceCount), peakTotalAggregation())
	if instOut.ok {
		m.MaxInstanceCount = &maxInst
	}
	cpu, cpuOut := fetchPercentile(
		ctx,
		client,
		t.ProjectID,
		window,
		cloudRunMetricFilter(t, crMetricCPUUtilization),
		monitoringpb.Aggregation_REDUCE_PERCENTILE_99,
	)
	if cpuOut.ok {
		m.CpuUtilizationP99 = &cpu
	}
	mem, memOut := fetchPercentile(
		ctx,
		client,
		t.ProjectID,
		window,
		cloudRunMetricFilter(t, crMetricMemoryUtilization),
		monitoringpb.Aggregation_REDUCE_PERCENTILE_99,
	)
	if memOut.ok {
		m.MemoryUtilizationP99 = &mem
	}
	startup, startupOut := fetchPercentile(
		ctx,
		client,
		t.ProjectID,
		window,
		cloudRunMetricFilter(t, crMetricStartupLatencies),
		monitoringpb.Aggregation_REDUCE_PERCENTILE_99,
	)
	if startupOut.ok {
		m.StartupLatencyP99 = &startup
	}

	outcomes := []metricOutcome{countOut, latOut}
	if countOut.ok {
		outcomes = append(outcomes, rateOut)
	}
	outcomes = append(outcomes, instOut, cpuOut, memOut, startupOut)
	partial, err := mergeOutcomes(outcomes...)
	return m, partial, err
}

// orZero reads "no series" as a zero count. Use it only for filtered error
// subsets, where an empty result means no errors happened. Totals keep
// "no data" so a wrong target name is still reported.
func orZero(o metricOutcome) metricOutcome {
	if errors.Is(o.err, errNoData) {
		return metricOutcome{ok: true}
	}
	return o
}

// error5xxRate divides the 5xx request_count subset by total, the
// request_count already fetched for the snapshot.
func error5xxRate(
	ctx context.Context,
	client MetricClient,
	t CloudRunTarget,
	window ObservationWindow,
	total int64,
) (float64, metricOutcome) {
	if total == 0 {
		// Zero request_count means no traffic; report 0% rather than dividing by zero.
		return 0, metricOutcome{ok: true}
	}
	n5xx, out := fetchSum(
		ctx,
		client,
		t.ProjectID,
		window,
		cloudRunMetricFilter(t, crMetricRequestCount, `metric.labels.response_code_class="5xx"`),
	)
	out = orZero(out)
	if out.err != nil {
		return 0, metricOutcome{err: fmt.Errorf("error_5xx_rate: %w", out.err)}
	}
	return float64(n5xx) / float64(total), metricOutcome{ok: true}
}

// sumAggregation aligns DELTA/CUMULATIVE counters with ALIGN_SUM + REDUCE_SUM.
func sumAggregation() *monitoringpb.Aggregation {
	return &monitoringpb.Aggregation{
		AlignmentPeriod:    durationpb.New(alignmentPeriod),
		PerSeriesAligner:   monitoringpb.Aggregation_ALIGN_SUM,
		CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_SUM,
	}
}

// peakTotalAggregation takes each series' per-minute max (ALIGN_MAX) and sums
// the series (REDUCE_SUM), so a metric split by a label such as instance_count's
// state (active, idle) yields one total per minute; fetchMax then keeps the
// highest minute.
func peakTotalAggregation() *monitoringpb.Aggregation {
	return &monitoringpb.Aggregation{
		AlignmentPeriod:    durationpb.New(alignmentPeriod),
		PerSeriesAligner:   monitoringpb.Aggregation_ALIGN_MAX,
		CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_SUM,
	}
}

// maxAggregation aligns GAUGE metrics with ALIGN_MAX + REDUCE_MAX.
func maxAggregation() *monitoringpb.Aggregation {
	return &monitoringpb.Aggregation{
		AlignmentPeriod:    durationpb.New(alignmentPeriod),
		PerSeriesAligner:   monitoringpb.Aggregation_ALIGN_MAX,
		CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_MAX,
	}
}

// maxAlignmentPeriod caps the percentile alignment period. The Monitoring
// Aggregation reference allows 104 weeks for charts and 90,000s for alerting
// policies and does not name ListTimeSeries, so the lower limit is used.
// Source: https://docs.cloud.google.com/monitoring/api/ref_v3/rpc/google.monitoring.v3#aggregation
const maxAlignmentPeriod = 90000 * time.Second

// windowPercentileAggregation merges each series' histograms over the whole
// window (ALIGN_SUM with one alignment period) and then takes the
// cross-series percentile, so Monitoring returns one window-wide value. It
// reports whether the period was clamped to maxAlignmentPeriod.
func windowPercentileAggregation(reducer monitoringpb.Aggregation_Reducer, window ObservationWindow) (*monitoringpb.Aggregation, bool) {
	period := window.End.Sub(window.Start)
	clamped := period > maxAlignmentPeriod
	if clamped {
		period = maxAlignmentPeriod
	}
	return &monitoringpb.Aggregation{
		AlignmentPeriod:    durationpb.New(period),
		PerSeriesAligner:   monitoringpb.Aggregation_ALIGN_SUM,
		CrossSeriesReducer: reducer,
	}, clamped
}

// errNoData marks a query that returned no matching series. It is wrapped
// with the metric filter so FetchMessage text reads "<filter>: no data".
var errNoData = errors.New("no data")

// noData returns the "no series" error for the named metric.
func noData(name string) error {
	return fmt.Errorf("%s: %w", name, errNoData)
}

// isQueryErr reports whether o failed with an API or context error, as
// opposed to returning no series.
func isQueryErr(o metricOutcome) bool {
	return o.err != nil && !errors.Is(o.err, errNoData)
}

// fetchSum queries a counter metric with sumAggregation and totals every point.
func fetchSum(ctx context.Context, client MetricClient, projectID string, window ObservationWindow, filter string) (int64, metricOutcome) {
	series, err := querySeries(ctx, client, projectID, window, filter, sumAggregation())
	if err != nil {
		return 0, metricOutcome{err: err}
	}
	v, ok := sumInt64Points(series)
	if !ok {
		return 0, metricOutcome{err: noData(filter)}
	}
	return v, metricOutcome{ok: true}
}

// fetchMax queries an INT64 or DOUBLE GAUGE metric with agg and keeps the
// highest point.
func fetchMax(
	ctx context.Context,
	client MetricClient,
	projectID string,
	window ObservationWindow,
	filter string,
	agg *monitoringpb.Aggregation,
) (float64, metricOutcome) {
	series, err := querySeries(ctx, client, projectID, window, filter, agg)
	if err != nil {
		return 0, metricOutcome{err: err}
	}
	v, ok := maxPoints(series)
	if !ok {
		return 0, metricOutcome{err: noData(filter)}
	}
	return v, metricOutcome{ok: true}
}

// fetchPercentile queries one window-wide percentile of a DELTA
// DISTRIBUTION metric. window must already be rounded to whole minutes.
func fetchPercentile(
	ctx context.Context,
	client MetricClient,
	projectID string,
	window ObservationWindow,
	filter string,
	reducer monitoringpb.Aggregation_Reducer,
) (float64, metricOutcome) {
	agg, clamped := windowPercentileAggregation(reducer, window)
	series, err := querySeries(ctx, client, projectID, window, filter, agg)
	if err != nil {
		return 0, metricOutcome{err: err}
	}
	v, ok := maxPoints(series)
	if !ok {
		return 0, metricOutcome{err: noData(filter)}
	}
	out := metricOutcome{ok: true}
	if clamped && pointCount(series) > 1 {
		// String concatenation, not fmt.Sprintf: perfsprint flags a lone %s.
		out.partial = []string{filter + ": window longer than 25h, percentile is the highest 25h value"}
	}
	return v, out
}

// fetchLatency queries p50, p95 and p99 of a latency distribution. It
// succeeds when at least one percentile returns a value.
func fetchLatency(
	ctx context.Context,
	client MetricClient,
	projectID string,
	window ObservationWindow,
	filter string,
) (*evalspb.LatencyPercentiles, metricOutcome) {
	p50, o50 := fetchPercentile(ctx, client, projectID, window, filter, monitoringpb.Aggregation_REDUCE_PERCENTILE_50)
	p95, o95 := fetchPercentile(ctx, client, projectID, window, filter, monitoringpb.Aggregation_REDUCE_PERCENTILE_95)
	p99, o99 := fetchPercentile(ctx, client, projectID, window, filter, monitoringpb.Aggregation_REDUCE_PERCENTILE_99)
	if !o50.ok && !o95.ok && !o99.ok {
		if !isQueryErr(o50) && !isQueryErr(o95) && !isQueryErr(o99) {
			return nil, metricOutcome{err: noData(filter)}
		}
		return nil, metricOutcome{err: joinErrors(o50.err, o95.err, o99.err)}
	}
	var missing []string
	for _, p := range []struct {
		name string
		out  metricOutcome
	}{{"p50", o50}, {"p95", o95}, {"p99", o99}} {
		if !p.out.ok {
			missing = append(missing, p.name)
		}
	}
	out := metricOutcome{ok: true}
	if len(missing) > 0 {
		out.partial = []string{fmt.Sprintf("%s: missing percentiles %s", filter, strings.Join(missing, ", "))}
	}
	return &evalspb.LatencyPercentiles{P50Ms: p50, P95Ms: p95, P99Ms: p99}, out
}
