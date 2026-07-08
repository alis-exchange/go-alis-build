# Retry

The `retry` package provides a generic, exponential-backoff retry helper with jitter,
context cancellation, and support for explicitly non-retryable errors.

## Install

```go
import "go.alis.build/utils/retry"
```

## Usage

### Basic

`Retry` invokes `f` up to `attempts` times, sleeping `~baseSleep * 2^i` (with full
jitter) between attempts.

```go
result, err := retry.Retry(3, 500*time.Millisecond, func() (Foo, error) {
    return callSomething()
})
```

### Context cancellation

Use `RetryContext` when you want backoff sleeps to abort as soon as a context is
cancelled. The context is also passed to `f`.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := retry.RetryContext(ctx, 5, 200*time.Millisecond,
    func(ctx context.Context) (Foo, error) {
        return callSomething(ctx)
    })
```

### Error-only variants

For functions that return only an `error`, use `Do` or `DoContext`:

```go
err := retry.Do(3, time.Second, func() error {
    return sendEvent()
})
```

### Options

- `retry.WithMaxSleep(max)` — cap the pre-jitter backoff between attempts.
- `retry.WithOnRetry(fn)` — callback invoked after each failed attempt that will be
  retried. Useful for logging or metrics.

```go
err := retry.Do(6, 100*time.Millisecond, sendEvent,
    retry.WithMaxSleep(2*time.Second),
    retry.WithOnRetry(func(attempt int, err error, nextSleep time.Duration) {
        log.Printf("attempt %d failed: %v (retrying in %v)", attempt, err, nextSleep)
    }),
)
```

### Non-retryable errors

Wrap an error with `NewNonRetryableError` to short-circuit the retry loop:

```go
result, err := retry.Retry(5, time.Second, func() (Foo, error) {
    res, err := callSomething()
    if errors.Is(err, ErrPermissionDenied) {
        return res, retry.NewNonRetryableError(err)
    }
    return res, err
})
```

`errors.Is` and `errors.As` traverse `NonRetryableError` via its `Unwrap` method,
so the original error remains inspectable by the caller.

## Backoff

The nominal delay before the `(i+1)`-th retry is `baseSleep * 2^i`, capped at
`WithMaxSleep` (if set) and at a safety limit that prevents `time.Duration`
overflow. The actual delay is drawn uniformly from `[0, nominal]` — the AWS
"full jitter" strategy — to avoid thundering-herd behavior across many clients.
