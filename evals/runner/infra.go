package runner

import (
	"context"

	"go.alis.build/evals/loadinfra"
)

// attachInfraClient lazily constructs one MetricClient per suite run when
// infra targets are configured. The returned close function must run after all
// cases in the run complete.
func attachInfraClient(ctx context.Context, cloud []loadinfra.CloudRunTarget, spanner []loadinfra.SpannerTarget) (context.Context, func(), error) {
	if len(cloud) == 0 && len(spanner) == 0 {
		return ctx, func() {}, nil
	}
	client, err := loadinfra.NewMetricClient(ctx)
	if err != nil {
		return ctx, func() {}, ErrInfraMetricClient{Err: err}
	}
	return loadinfra.WithClient(ctx, client), func() { _ = client.Close() }, nil
}
