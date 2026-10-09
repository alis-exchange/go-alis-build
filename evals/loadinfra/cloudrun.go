package loadinfra

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	rate, rateOut := fetchError5xxRate(ctx, client, t, window)
	if rateOut.ok {
		m.Error_5XxRate = &rate
	}
	maxInst, instOut := fetchMax(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricInstanceCount))
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

	partial, err := mergeOutcomes(countOut, latOut, rateOut, instOut, cpuOut, memOut, startupOut)
	return m, partial, err
}

// fetchError5xxRate derives the 5xx fraction from request_count series
// filtered by response_code_class="5xx" over the same window as the total.
func fetchError5xxRate(ctx context.Context, client MetricClient, t CloudRunTarget, window ObservationWindow) (float64, metricOutcome) {
	total, totalOut := fetchSum(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricRequestCount))
	if errors.Is(totalOut.err, errNoData) {
		return 0, metricOutcome{err: fmt.Errorf("error_5xx_rate: %w", errNoData)}
	}
	if totalOut.err != nil {
		return 0, metricOutcome{err: fmt.Errorf("error_5xx_rate: %w", totalOut.err)}
	}
	if total == 0 {
		// Zero request_count means no traffic; report 0% rather than dividing by zero.
		return 0, metricOutcome{ok: true}
	}
	n5xx, errOut := fetchSum(
		ctx,
		client,
		t.ProjectID,
		window,
		cloudRunMetricFilter(t, crMetricRequestCount, `metric.labels.response_code_class="5xx"`),
	)
	if isQueryErr(errOut) {
		return 0, metricOutcome{err: fmt.Errorf("error_5xx_rate: %w", errOut.err)}
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

// maxAggregation aligns GAUGE metrics with ALIGN_MAX + REDUCE_MAX.
func maxAggregation() *monitoringpb.Aggregation {
	return &monitoringpb.Aggregation{
		AlignmentPeriod:    durationpb.New(alignmentPeriod),
		PerSeriesAligner:   monitoringpb.Aggregation_ALIGN_MAX,
		CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_MAX,
	}
}

// distributionPercentileAggregation builds the Monitoring aggregation for
// DELTA DISTRIBUTION metrics (request_latencies, cpu/memory utilizations).
func distributionPercentileAggregation(reducer monitoringpb.Aggregation_Reducer) *monitoringpb.Aggregation {
	// DELTA DISTRIBUTION metrics (request_latencies, cpu/memory utilizations):
	// merge histograms per series per minute (ALIGN_SUM), then compute the
	// cross-series percentile (REDUCE_PERCENTILE_*). This matches the supported
	// Monitoring API path for distribution metrics.
	return &monitoringpb.Aggregation{
		AlignmentPeriod:    durationpb.New(alignmentPeriod),
		PerSeriesAligner:   monitoringpb.Aggregation_ALIGN_SUM,
		CrossSeriesReducer: reducer,
	}
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

// fetchMax queries a GAUGE metric with maxAggregation and keeps the highest point.
func fetchMax(
	ctx context.Context,
	client MetricClient,
	projectID string,
	window ObservationWindow,
	filter string,
) (float64, metricOutcome) {
	series, err := querySeries(ctx, client, projectID, window, filter, maxAggregation())
	if err != nil {
		return 0, metricOutcome{err: err}
	}
	v, ok := maxDoublePoints(series)
	if !ok {
		return 0, metricOutcome{err: noData(filter)}
	}
	return v, metricOutcome{ok: true}
}

// fetchPercentile queries a DELTA DISTRIBUTION metric for one percentile and
// returns the mean of the returned points.
func fetchPercentile(
	ctx context.Context,
	client MetricClient,
	projectID string,
	window ObservationWindow,
	filter string,
	reducer monitoringpb.Aggregation_Reducer,
) (float64, metricOutcome) {
	series, err := querySeries(ctx, client, projectID, window, filter, distributionPercentileAggregation(reducer))
	if err != nil {
		return 0, metricOutcome{err: err}
	}
	v, ok := meanDoublePoints(series)
	if !ok {
		return 0, metricOutcome{err: noData(filter)}
	}
	return v, metricOutcome{ok: true}
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
	if isQueryErr(o50) && isQueryErr(o95) && isQueryErr(o99) {
		return nil, metricOutcome{err: o50.err}
	}
	if !o50.ok && !o95.ok && !o99.ok {
		return nil, metricOutcome{err: noData(filter)}
	}
	return &evalspb.LatencyPercentiles{P50Ms: p50, P95Ms: p95, P99Ms: p99}, metricOutcome{ok: true}
}
