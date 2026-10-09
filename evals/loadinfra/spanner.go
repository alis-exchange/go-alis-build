package loadinfra

import (
	"context"
	"fmt"
	"strings"

	evalspb "go.alis.build/common/alis/evals"
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
func fetchSpannerMetrics(
	ctx context.Context,
	client MetricClient,
	t SpannerTarget,
	window ObservationWindow,
) (*evalspb.SpannerMetrics, []string, error) {
	m := &evalspb.SpannerMetrics{}

	total, totalOut := fetchSum(ctx, client, t.ProjectID, window, spannerMetricFilter(t, spMetricQueryCount))
	if totalOut.ok {
		m.QueryCount = total
	}
	// The failed-query subset reads "no series" as zero errors, which is only
	// meaningful when the total returned data. Without a total it adds nothing,
	// so a wrong target name is still reported as unavailable.
	var errOut metricOutcome
	if totalOut.ok {
		var errCount int64
		errCount, errOut = fetchSum(
			ctx,
			client,
			t.ProjectID,
			window,
			spannerMetricFilter(t, spMetricQueryCount, `metric.labels.status!="ok"`),
		)
		errOut = orZero(errOut)
		if errOut.ok {
			m.QueryErrorCount = errCount
		}
	}
	apiLat, apiOut := fetchSpannerLatency(ctx, client, t.ProjectID, window, spannerMetricFilter(t, spMetricAPILatencies))
	if apiOut.ok {
		m.ApiLatency = apiLat
	}
	queryLat, queryOut := fetchSpannerLatency(ctx, client, t.ProjectID, window, spannerMetricFilter(t, spMetricQueryLatencies))
	if queryOut.ok {
		m.QueryLatency = queryLat
	}
	cpuFilter := strings.Join([]string{
		spannerResourceFilter(t),
		fmt.Sprintf(`metric.type="%s"`, spMetricInstanceCPU),
	}, " AND ")
	cpu, cpuOut := fetchMax(ctx, client, t.ProjectID, window, cpuFilter)
	if cpuOut.ok {
		m.CpuUtilizationMax = &cpu
	}

	outcomes := []metricOutcome{totalOut}
	if totalOut.ok {
		outcomes = append(outcomes, errOut)
	}
	outcomes = append(outcomes, apiOut, queryOut, cpuOut)
	partial, err := mergeOutcomes(outcomes...)
	return m, partial, err
}

// fetchSpannerLatency queries Spanner latency distributions, which are
// reported in seconds, and converts percentiles to milliseconds.
func fetchSpannerLatency(
	ctx context.Context,
	client MetricClient,
	projectID string,
	window ObservationWindow,
	filter string,
) (*evalspb.LatencyPercentiles, metricOutcome) {
	lat, out := fetchLatency(ctx, client, projectID, window, filter)
	if !out.ok {
		return nil, out
	}
	return &evalspb.LatencyPercentiles{
		P50Ms: lat.P50Ms * 1000,
		P95Ms: lat.P95Ms * 1000,
		P99Ms: lat.P99Ms * 1000,
	}, out
}
