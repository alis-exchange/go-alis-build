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

// maxPoints returns the maximum numeric value across all points in all
// returned series. Each point is read by its value type, so INT64 gauges
// such as Cloud Run container/instance_count and DOUBLE gauges such as
// Spanner instance/cpu/utilization both work. Points of any other value
// type are skipped.
func maxPoints(series []*monitoringpb.TimeSeries) (float64, bool) {
	var max float64
	var found bool
	for _, ts := range series {
		for _, p := range ts.Points {
			v, ok := numericValue(p.GetValue())
			if !ok {
				continue
			}
			if !found || v > max {
				max = v
				found = true
			}
		}
	}
	return max, found
}

// numericValue reads a DOUBLE or INT64 typed value as float64. It reports
// false for nil and for any other value type.
func numericValue(v *monitoringpb.TypedValue) (float64, bool) {
	switch x := v.GetValue().(type) {
	case *monitoringpb.TypedValue_DoubleValue:
		return x.DoubleValue, true
	case *monitoringpb.TypedValue_Int64Value:
		return float64(x.Int64Value), true
	default:
		return 0, false
	}
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
