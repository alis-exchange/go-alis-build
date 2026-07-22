package runner

import (
	"context"

	"go.alis.build/evals/loadinfra"
)

// MetricClientFactory constructs a MetricClient for one suite run. Tests inject
// fakes here; production leaves the factory nil and [loadinfra.NewMetricClient]
// is used instead.
type MetricClientFactory func(context.Context) (loadinfra.MetricClient, error)

// attachInfraClient lazily constructs one MetricClient per suite run when
// infra targets are configured. The returned close function must run after all
// cases in the run complete. When a client is already on ctx, close is a no-op
// and the caller retains ownership.
func attachInfraClient(ctx context.Context, cloud []loadinfra.CloudRunTarget, spanner []loadinfra.SpannerTarget, factory MetricClientFactory) (context.Context, func(), error) {
	if len(cloud) == 0 && len(spanner) == 0 {
		return ctx, func() {}, nil
	}
	if existing := loadinfra.ClientFromContext(ctx); existing != nil {
		return ctx, func() {}, nil
	}
	var (
		client loadinfra.MetricClient
		err    error
	)
	if factory != nil {
		client, err = factory(ctx)
	} else {
		client, err = loadinfra.NewMetricClient(ctx)
	}
	if err != nil {
		return ctx, func() {}, ErrInfraMetricClient{Err: err}
	}
	return loadinfra.WithClient(ctx, client), func() { _ = client.Close() }, nil
}
