package evals

import (
	"context"
	"time"
)

// Result is what Call returns: the typed response, transport error (if any),
// and wall-clock latency. Assertions are the author's responsibility; Call
// records nothing on T.
type Result[T any] struct {
	Resp    T
	Err     error
	Latency time.Duration
}

// Call invokes fn and captures the response, error, and wall-clock latency.
// The type parameter is inferred from fn, so authors never write it explicitly.
//
//	r := evals.Call(ctx, func(ctx context.Context) (*filespb.File, error) {
//	    return clients.Files.GetFile(ctx, req)
//	})
//	if !t.NoErr("grpc", r.Err) { return }
//	t.Max("latency", r.Latency, budget)
func Call[T any](ctx context.Context, fn func(context.Context) (T, error)) Result[T] {
	start := time.Now()
	resp, err := fn(ctx)
	return Result[T]{Resp: resp, Err: err, Latency: time.Since(start)}
}
