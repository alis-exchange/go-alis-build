package loadinfra

import (
	"context"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.alis.build/alog"
	"google.golang.org/api/iterator"
)

// MetricClient queries Cloud Monitoring time series. Production code uses
// NewMetricClient; tests inject fakes at this boundary.
type MetricClient interface {
	QueryTimeSeries(ctx context.Context, projectID string, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error)
	Close() error
}

// NewMetricClient constructs a production MetricClient backed by the Cloud
// Monitoring API.
func NewMetricClient(ctx context.Context) (MetricClient, error) {
	c, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}
	return &googleMetricClient{client: c}, nil
}

// googleMetricClient adapts the Cloud Monitoring client to MetricClient.
type googleMetricClient struct {
	// client is the underlying generated Monitoring API client.
	client *monitoring.MetricClient
}

// QueryTimeSeries lists time series for projectID using req.Filter and req.Aggregation.
func (c *googleMetricClient) QueryTimeSeries(ctx context.Context, projectID string, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
	req.Name = "projects/" + projectID
	it := c.client.ListTimeSeries(ctx, req)
	var out []*monitoringpb.TimeSeries
	for {
		ts, err := it.Next()
		if err == iterator.Done {
			// iterator.Done means no (more) matching time series — not an API error.
			return out, nil
		}
		if err != nil {
			alog.Errorf(ctx, "failed to list time series: %v", err)
			return nil, err
		}
		out = append(out, ts)
	}
}

// Close releases the underlying Monitoring client.
func (c *googleMetricClient) Close() error {
	return c.client.Close()
}

// FakeMetricClient records queries and returns canned time series keyed by the
// ListTimeSeriesRequest filter string.
type FakeMetricClient struct {
	// ByFilter maps Monitoring filter strings to canned time series responses.
	ByFilter map[string][]*monitoringpb.TimeSeries
	// Err is returned from every QueryTimeSeries call when non-nil.
	Err error
	// Calls counts how many QueryTimeSeries invocations were made.
	Calls int
	// LastIntervalEnd records the EndTime from the most recent request.
	LastIntervalEnd time.Time
}

// QueryTimeSeries returns canned series from ByFilter or Err when configured.
func (f *FakeMetricClient) QueryTimeSeries(ctx context.Context, projectID string, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
	f.Calls++
	if req.Interval != nil && req.Interval.EndTime != nil {
		f.LastIntervalEnd = req.Interval.EndTime.AsTime()
	}
	if f.Err != nil {
		return nil, f.Err
	}
	if f.ByFilter == nil {
		return nil, nil
	}
	return f.ByFilter[req.Filter], nil
}

func (f *FakeMetricClient) Close() error { return nil } // no-op for tests.
