package loadinfratest

import "go.alis.build/evals/loadinfra"

// MetricClient records queries and returns canned time series keyed by the
// ListTimeSeriesRequest filter string. Set Handler to answer per request.
// It is safe for concurrent use.
type MetricClient = loadinfra.FakeMetricClient //nolint:staticcheck // SA1019: this alias is the supported name for the deprecated struct.
