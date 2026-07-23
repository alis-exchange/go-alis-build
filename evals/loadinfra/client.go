package loadinfra

import (
	"context"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.alis.build/alog"
	"google.golang.org/api/iterator"
)

type clientContextKey struct{}

// WithClient attaches a MetricClient to ctx for load case execution.
func WithClient(ctx context.Context, client MetricClient) context.Context {
	return context.WithValue(ctx, clientContextKey{}, client)
}

// ClientFromContext returns the MetricClient attached by [WithClient], or nil.
func ClientFromContext(ctx context.Context) MetricClient {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(clientContextKey{}).(MetricClient)
	return c
}

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
	// CloseCalls counts how many Close invocations were made.
	CloseCalls int
	// BlockDelay sleeps on each QueryTimeSeries call when non-zero.
	BlockDelay time.Duration
	// PeakInFlight records the maximum concurrent QueryTimeSeries calls.
	PeakInFlight int
	// LastIntervalEnd records the EndTime from the most recent request.
	LastIntervalEnd time.Time

	mu       sync.Mutex
	inFlight int
}

// QueryTimeSeries returns canned series from ByFilter or Err when configured.
func (f *FakeMetricClient) QueryTimeSeries(ctx context.Context, projectID string, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
	f.mu.Lock()
	f.Calls++
	f.inFlight++
	if f.inFlight > f.PeakInFlight {
		f.PeakInFlight = f.inFlight
	}
	delay := f.BlockDelay
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			f.mu.Lock()
			f.inFlight--
			f.mu.Unlock()
			return nil, ctx.Err()
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if req.Interval != nil && req.Interval.EndTime != nil {
		f.LastIntervalEnd = req.Interval.EndTime.AsTime()
	}
	if f.Err != nil {
		f.inFlight--
		return nil, f.Err
	}
	if f.ByFilter == nil {
		f.inFlight--
		return nil, nil
	}
	series := f.ByFilter[req.Filter]
	f.inFlight--
	return series, nil
}

func (f *FakeMetricClient) Close() error {
	f.CloseCalls++
	return nil
}
