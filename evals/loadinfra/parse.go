package loadinfra

import (
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
)

const (
	// alignmentPeriod is the 60s period used for counter and gauge queries;
	// Monitoring requires at least 60s.
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

// pointCount counts points with a value across all series.
func pointCount(series []*monitoringpb.TimeSeries) int {
	var n int
	for _, ts := range series {
		for _, p := range ts.Points {
			if p.Value != nil {
				n++
			}
		}
	}
	return n
}
