package loadinfra

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	evalspb "go.alis.build/common/alis/evals/v1"
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
func fetchCloudRunMetrics(ctx context.Context, client MetricClient, t CloudRunTarget, window ObservationWindow) (*evalspb.CloudRunMetrics, []string, error) {
	// Each metric is fetched independently; partial failures still emit OK snapshots
	// with FetchMessage listing missing series.
	m := &evalspb.CloudRunMetrics{}

	countOutcome := fetchInt64Sum(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricRequestCount))
	if countOutcome.ok {
		m.RequestCount = countOutcome.value
	}

	latOutcome := fetchLatencyPercentiles(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricRequestLatencies))
	if latOutcome.ok {
		m.Latency = latOutcome.latency
	}

	err5xxOutcome := fetchError5xxRate(ctx, client, t, window)
	if err5xxOutcome.ok && err5xxOutcome.value > 0 {
		m.Error_5XxRate = &err5xxOutcome.value
	} else if err5xxOutcome.ok {
		zero := 0.0
		m.Error_5XxRate = &zero
	}

	maxInstOutcome := fetchDoubleMax(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricInstanceCount))
	if maxInstOutcome.ok {
		m.MaxInstanceCount = &maxInstOutcome.value
	}

	cpuOutcome := fetchDistributionPercentile(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricCPUUtilization), monitoringpb.Aggregation_REDUCE_PERCENTILE_99)
	if cpuOutcome.ok {
		m.CpuUtilizationP99 = &cpuOutcome.value
	}

	memOutcome := fetchDistributionPercentile(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricMemoryUtilization), monitoringpb.Aggregation_REDUCE_PERCENTILE_99)
	if memOutcome.ok {
		m.MemoryUtilizationP99 = &memOutcome.value
	}

	startupOutcome := fetchDistributionPercentile(ctx, client, t.ProjectID, window, cloudRunMetricFilter(t, crMetricStartupLatencies), monitoringpb.Aggregation_REDUCE_PERCENTILE_99)
	if startupOutcome.ok {
		m.StartupLatencyP99 = &startupOutcome.value
	}

	partial, err := mergeOutcomes(
		countOutcome.outcome(),
		latOutcome.outcome(),
		err5xxOutcome.outcome(),
		maxInstOutcome.outcome(),
		cpuOutcome.outcome(),
		memOutcome.outcome(),
		startupOutcome.outcome(),
	)
	return m, partial, err
}

// fetchError5xxRate derives the 5xx fraction from request_count series filtered
// by response_code_class="5xx" over the same window as total request_count.
func fetchError5xxRate(ctx context.Context, client MetricClient, t CloudRunTarget, window ObservationWindow) error5xxOutcome {
	totalFilter := cloudRunMetricFilter(t, crMetricRequestCount)
	errFilter := cloudRunMetricFilter(t, crMetricRequestCount, `metric.labels.response_code_class="5xx"`)

	total := fetchInt64Sum(ctx, client, t.ProjectID, window, totalFilter)
	if total.err != nil {
		return error5xxOutcome{err: fmt.Errorf("error_5xx_rate: %w", total.err)}
	}
	if !total.ok {
		return error5xxOutcome{err: fmt.Errorf("error_5xx_rate: no data")}
	}
	if total.value == 0 {
		// Zero request_count means no traffic in the window; report 0% rather than dividing by zero.
		return error5xxOutcome{ok: true, value: 0}
	}
	err5xx := fetchInt64Sum(ctx, client, t.ProjectID, window, errFilter)
	if err5xx.err != nil {
		return error5xxOutcome{err: fmt.Errorf("error_5xx_rate: %w", err5xx.err)}
	}
	rate := float64(err5xx.value) / float64(total.value)
	return error5xxOutcome{ok: true, value: rate}
}

// error5xxOutcome holds the derived 5xx rate or a fetch error.
type error5xxOutcome struct {
	// ok is true when rate was computed (including zero traffic).
	ok bool
	// value is 5xx count divided by total request_count.
	value float64
	// err is non-nil when either underlying query failed.
	err error
}

// outcome adapts error5xxOutcome into the shared mergeOutcomes input shape.
func (o error5xxOutcome) outcome() metricOutcome {
	if o.err != nil {
		return metricOutcome{err: o.err}
	}
	if !o.ok {
		return metricOutcome{err: fmt.Errorf("error_5xx_rate: no data")}
	}
	return metricOutcome{ok: true}
}

// outcome adapts int64Outcome into the shared mergeOutcomes input shape.
func (o int64Outcome) outcome() metricOutcome {
	if o.err != nil {
		return metricOutcome{err: o.err}
	}
	if !o.ok {
		return metricOutcome{err: fmt.Errorf("%s: no data", o.name)}
	}
	return metricOutcome{ok: true}
}

// outcome adapts latencyOutcome into the shared mergeOutcomes input shape.
func (o latencyOutcome) outcome() metricOutcome {
	if o.err != nil {
		return metricOutcome{err: o.err}
	}
	if !o.ok {
		return metricOutcome{err: fmt.Errorf("%s: no data", o.name)}
	}
	return metricOutcome{ok: true}
}

// outcome adapts doubleOutcome into the shared mergeOutcomes input shape.
func (o doubleOutcome) outcome() metricOutcome {
	if o.err != nil {
		return metricOutcome{err: o.err}
	}
	if !o.ok {
		return metricOutcome{err: fmt.Errorf("%s: no data", o.name)}
	}
	return metricOutcome{ok: true}
}

// int64Outcome holds a summed INT64 metric result or a fetch error.
type int64Outcome struct {
	// name is the Monitoring filter string used in diagnostic messages.
	name string
	// ok is true when at least one matching time series was returned.
	ok bool
	// value is the aggregated total after ALIGN_SUM + REDUCE_SUM.
	value int64
	// err is non-nil when ListTimeSeries failed.
	err error
}

// doubleOutcome holds a scalar DOUBLE metric result or a fetch error.
type doubleOutcome struct {
	// name is the Monitoring filter string used in diagnostic messages.
	name string
	// ok is true when at least one matching time series was returned.
	ok bool
	// value is the aggregated scalar after alignment and reduction.
	value float64
	// err is non-nil when ListTimeSeries failed.
	err error
}

// latencyOutcome holds p50/p95/p99 latency percentiles or a fetch error.
type latencyOutcome struct {
	// name is the Monitoring filter string used in diagnostic messages.
	name string
	// ok is true when at least one percentile succeeded.
	ok bool
	// latency is populated when ok; partial percentiles are allowed.
	latency *evalspb.LatencyPercentiles
	// err is non-nil when every percentile query failed.
	err error
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

// fetchDistributionPercentile queries a DELTA DISTRIBUTION metric and returns
// the mean of per-series percentile scalars after cross-series reduction.
func fetchDistributionPercentile(ctx context.Context, client MetricClient, projectID string, window ObservationWindow, filter string, reducer monitoringpb.Aggregation_Reducer) doubleOutcome {
	series, err := querySeries(ctx, client, projectID, window, filter, distributionPercentileAggregation(reducer))
	if err != nil {
		return doubleOutcome{name: filter, err: err}
	}
	v, ok := meanDoublePoints(series)
	return doubleOutcome{name: filter, ok: ok, value: v}
}

// fetchInt64Sum queries a counter metric with sumAggregation.
func fetchInt64Sum(ctx context.Context, client MetricClient, projectID string, window ObservationWindow, filter string) int64Outcome {
	series, err := querySeries(ctx, client, projectID, window, filter, sumAggregation())
	if err != nil {
		return int64Outcome{name: filter, err: err}
	}
	v, ok := sumInt64Points(series)
	return int64Outcome{name: filter, ok: ok, value: v}
}

// fetchDoubleMax queries a GAUGE metric with maxAggregation.
func fetchDoubleMax(ctx context.Context, client MetricClient, projectID string, window ObservationWindow, filter string) doubleOutcome {
	series, err := querySeries(ctx, client, projectID, window, filter, maxAggregation())
	if err != nil {
		return doubleOutcome{name: filter, err: err}
	}
	v, ok := maxDoublePoints(series)
	return doubleOutcome{name: filter, ok: ok, value: v}
}

// fetchDoublePercentile is an alias for fetchDistributionPercentile.
func fetchDoublePercentile(ctx context.Context, client MetricClient, projectID string, window ObservationWindow, filter string, reducer monitoringpb.Aggregation_Reducer) doubleOutcome {
	return fetchDistributionPercentile(ctx, client, projectID, window, filter, reducer)
}

// fetchLatencyPercentiles queries p50/p95/p99 for a DELTA DISTRIBUTION latency
// metric. Partial percentile gaps are tolerated when at least one succeeds.
func fetchLatencyPercentiles(ctx context.Context, client MetricClient, projectID string, window ObservationWindow, filter string) latencyOutcome {
	p50 := fetchDistributionPercentile(ctx, client, projectID, window, filter, monitoringpb.Aggregation_REDUCE_PERCENTILE_50)
	p95 := fetchDistributionPercentile(ctx, client, projectID, window, filter, monitoringpb.Aggregation_REDUCE_PERCENTILE_95)
	p99 := fetchDistributionPercentile(ctx, client, projectID, window, filter, monitoringpb.Aggregation_REDUCE_PERCENTILE_99)
	if p50.err != nil && p95.err != nil && p99.err != nil {
		return latencyOutcome{name: filter, err: p50.err}
	}
	if !p50.ok && !p95.ok && !p99.ok {
		return latencyOutcome{name: filter}
	}
	lat := &evalspb.LatencyPercentiles{}
	if p50.ok {
		lat.P50Ms = p50.value
	}
	if p95.ok {
		lat.P95Ms = p95.value
	}
	if p99.ok {
		lat.P99Ms = p99.value
	}
	return latencyOutcome{name: filter, ok: true, latency: lat}
}
