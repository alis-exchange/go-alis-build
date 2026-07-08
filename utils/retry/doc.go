// Package retry provides a generic, exponential-backoff retry helper with full
// jitter, context cancellation, and support for explicitly non-retryable errors.
//
// The main entry points are:
//
//   - [Retry]        — retry a function returning (R, error).
//   - [RetryContext] — same, with a context.Context passed to f and used to
//     cancel pending backoff sleeps.
//   - [Do]           — retry a function returning only an error.
//   - [DoContext]    — same, with context support.
//
// Behavior is customized via [Option] values, currently [WithMaxSleep] and
// [WithOnRetry].
//
// Backoff between the i-th and (i+1)-th attempt is nominally baseSleep * 2^i,
// optionally capped by [WithMaxSleep]. The actual sleep is drawn uniformly from
// [0, nominal] — the AWS "full jitter" strategy — to avoid thundering-herd
// behavior across many clients.
//
// To short-circuit the retry loop from within f, wrap the returned error with
// [NewNonRetryableError]. Retry stops immediately and returns the underlying
// error; errors.Is and errors.As traverse the wrapper via [NonRetryableError.Unwrap].
package retry
