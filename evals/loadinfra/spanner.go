package loadinfra

import (
	"context"
	"fmt"
	"strings"

	evalspb "go.alis.build/common/alis/evals/v1"
)

const (
	// spMetricQueryCount is the general per-database query counter (labels:
	// database, status, query_type, optimizer_version).
	spMetricQueryCount = "spanner.googleapis.com/query_count"
	// spMetricQueryLatencies is the query-stat latency distribution.
	spMetricQueryLatencies = "spanner.googleapis.com/query_stat/total/query_latencies"
	// spMetricAPILatencies is the Spanner API request latency distribution.
	spMetricAPILatencies = "spanner.googleapis.com/api/request_latencies"
	// spMetricInstanceCPU is instance-scoped CPU utilization (not attributable
	// to a single database).
	spMetricInstanceCPU = "spanner.googleapis.com/instance/cpu/utilization"
)

// spannerResourceFilter builds the resource clause shared by database-scoped
// Spanner metric queries for target t.
func spannerResourceFilter(t SpannerTarget) string {
	return strings.Join([]string{
		`resource.type="spanner_instance"`,
		fmt.Sprintf(`resource.labels.instance_id="%s"`, escapeFilterLabel(t.InstanceID)),
		fmt.Sprintf(`resource.labels.location="%s"`, escapeFilterLabel(t.Location)),
	}, " AND ")
}

// spannerMetricFilter combines the resource filter, metric type, database
// label, and optional extra predicates.
func spannerMetricFilter(t SpannerTarget, metricType string, extra ...string) string {
	parts := []string{
		spannerResourceFilter(t),
		fmt.Sprintf(`metric.type="%s"`, metricType),
		fmt.Sprintf(`metric.labels.database="%s"`, escapeFilterLabel(t.Database)),
	}
	parts = append(parts, extra...)
	return strings.Join(parts, " AND ")
}

// fetchSpannerMetrics queries all Spanner metrics for one target. CPU is
// instance-scoped and uses a filter without the database label.
func fetchSpannerMetrics(ctx context.Context, client MetricClient, t SpannerTarget, window ObservationWindow) (*evalspb.SpannerMetrics, []string, error) {
	m := &evalspb.SpannerMetrics{}

	totalOutcome := fetchInt64Sum(ctx, client, t.ProjectID, window, spannerMetricFilter(t, spMetricQueryCount))
	if totalOutcome.ok {
		m.QueryCount = totalOutcome.value
	}

	errOutcome := fetchInt64Sum(ctx, client, t.ProjectID, window, spannerMetricFilter(t, spMetricQueryCount, `metric.labels.status!="ok"`))
	if errOutcome.ok {
		m.QueryErrorCount = errOutcome.value
	}

	apiLatOutcome := fetchSpannerLatencyPercentiles(ctx, client, t.ProjectID, window, spannerMetricFilter(t, spMetricAPILatencies))
	if apiLatOutcome.ok {
		m.ApiLatency = apiLatOutcome.latency
	}

	queryLatOutcome := fetchSpannerLatencyPercentiles(ctx, client, t.ProjectID, window, spannerMetricFilter(t, spMetricQueryLatencies))
	if queryLatOutcome.ok {
		m.QueryLatency = queryLatOutcome.latency
	}

	cpuFilter := strings.Join([]string{
		spannerResourceFilter(t),
		fmt.Sprintf(`metric.type="%s"`, spMetricInstanceCPU),
	}, " AND ")
	cpuOutcome := fetchDoubleMax(ctx, client, t.ProjectID, window, cpuFilter)
	if cpuOutcome.ok {
		m.CpuUtilizationMax = &cpuOutcome.value
	}

	partial, err := mergeOutcomes(
		totalOutcome.outcome(),
		errOutcome.outcome(),
		apiLatOutcome.outcome(),
		queryLatOutcome.outcome(),
		cpuOutcome.outcome(),
	)
	return m, partial, err
}

// fetchSpannerLatencyPercentiles queries Spanner latency distributions, which
// are reported in seconds, and converts percentiles to milliseconds for proto
// LatencyPercentiles fields.
func fetchSpannerLatencyPercentiles(ctx context.Context, client MetricClient, projectID string, window ObservationWindow, filter string) latencyOutcome {
	out := fetchLatencyPercentiles(ctx, client, projectID, window, filter)
	if !out.ok || out.latency == nil {
		return out
	}
	lat := out.latency
	out.latency = &evalspb.LatencyPercentiles{
		P50Ms: lat.P50Ms * 1000,
		P95Ms: lat.P95Ms * 1000,
		P99Ms: lat.P99Ms * 1000,
	}
	return out
}
