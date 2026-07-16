package loadinfra

import "context"

// clientContextKey is the private context key for [WithClient] / [ClientFromContext].
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
