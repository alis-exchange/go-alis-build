package loadinfra

import (
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
)

const (
	// Cloud Monitoring requires alignment periods ≥60s for these metric kinds.
	alignmentPeriod = 60 * time.Second
)

// sumInt64Points totals INT64 values across all points in all returned series.
func sumInt64Points(series []*monitoringpb.TimeSeries) (int64, bool) {
	var total int64
	var found bool
	for _, ts := range series {
		for _, p := range ts.Points {
			if p.Value == nil {
				continue
			}
			total += p.Value.GetInt64Value()
			found = true
		}
	}
	return total, found
}

// maxDoublePoints returns the maximum DOUBLE value across all points in all
// returned series.
func maxDoublePoints(series []*monitoringpb.TimeSeries) (float64, bool) {
	var max float64
	var found bool
	for _, ts := range series {
		for _, p := range ts.Points {
			if p.Value == nil {
				continue
			}
			v := p.Value.GetDoubleValue()
			if !found || v > max {
				max = v
				found = true
			}
		}
	}
	return max, found
}

// meanDoublePoints averages scalar values across time series. After cross-series
// percentile reduction each series contributes one scalar; mean aggregates
// multi-series percentiles.
func meanDoublePoints(series []*monitoringpb.TimeSeries) (float64, bool) {
	var sum float64
	var n int
	for _, ts := range series {
		for _, p := range ts.Points {
			if p.Value == nil {
				continue
			}
			sum += p.Value.GetDoubleValue()
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}
