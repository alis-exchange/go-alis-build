package loadgen

import "context"

type abortOnSLOKey struct{}

// ContextWithAbortOnSLOFailure marks ctx so load cases install an abort
// check that cancels the generator when any declared SLO fails.
func ContextWithAbortOnSLOFailure(ctx context.Context) context.Context {
	return context.WithValue(ctx, abortOnSLOKey{}, true)
}

// AbortOnSLOFailure reports whether ctx was marked by
// [ContextWithAbortOnSLOFailure].
func AbortOnSLOFailure(ctx context.Context) bool {
	v, _ := ctx.Value(abortOnSLOKey{}).(bool)
	return v
}
