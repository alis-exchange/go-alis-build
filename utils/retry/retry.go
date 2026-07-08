package retry

import (
	"context"
	"errors"
	"math/bits"
	"math/rand/v2"
	"time"
)

// Option configures optional behavior of Retry, RetryContext, Do, and DoContext.
type Option func(*config)

type config struct {
	// maxSleep caps the pre-jitter backoff between attempts. Zero means no cap.
	maxSleep time.Duration
	// onRetry is invoked after each failed attempt that will be followed by a
	// sleep. attempt is the 1-indexed count of the failed attempt.
	onRetry func(attempt int, err error, nextSleep time.Duration)
}

// WithMaxSleep caps the pre-jitter backoff between attempts to maxSleep. A zero
// or negative value disables the cap.
func WithMaxSleep(maxSleep time.Duration) Option {
	return func(c *config) { c.maxSleep = maxSleep }
}

// WithOnRetry sets a callback invoked after each failed attempt that will be
// retried. attempt is the 1-indexed count of the just-failed attempt, err is
// the returned error, and nextSleep is the (jittered) sleep before the next
// attempt.
func WithOnRetry(fn func(attempt int, err error, nextSleep time.Duration)) Option {
	return func(c *config) { c.onRetry = fn }
}

// Retry invokes f up to attempts times with exponential backoff and full jitter.
// It returns the result of the first successful call, or the last error if every
// attempt fails.
//
// The nominal backoff between the i-th and (i+1)-th call is baseSleep * 2^i,
// optionally capped via WithMaxSleep. The actual sleep is drawn uniformly from
// [0, nominal] (full jitter) to avoid thundering herds.
//
// If attempts is less than 1, f is still invoked once.
//
// If the error returned by f is (or wraps) a NonRetryableError, Retry stops
// immediately and returns the underlying error for later checking.
func Retry[R any](attempts int, baseSleep time.Duration, f func() (R, error), opts ...Option) (R, error) {
	return RetryContext(context.Background(), attempts, baseSleep, func(context.Context) (R, error) {
		return f()
	}, opts...)
}

// RetryContext is like Retry but passes ctx to f and aborts pending sleeps as
// soon as ctx is cancelled. When ctx is cancelled during a backoff sleep,
// RetryContext returns the last error from f.
func RetryContext[R any](ctx context.Context, attempts int, baseSleep time.Duration, f func(context.Context) (R, error), opts ...Option) (R, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	if attempts < 1 {
		attempts = 1
	}

	var (
		res R
		err error
	)

	for i := 0; i < attempts; i++ {
		res, err = f(ctx)
		if err == nil {
			return res, nil
		}

		var nre NonRetryableError
		if errors.As(err, &nre) {
			if inner := nre.Unwrap(); inner != nil {
				return res, inner
			}
			return res, err
		}

		if i == attempts-1 {
			break
		}

		sleep := backoff(baseSleep, i, cfg.maxSleep)

		if cfg.onRetry != nil {
			cfg.onRetry(i+1, err, sleep)
		}

		if sleep <= 0 {
			continue
		}

		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return res, errors.Join(err, ctx.Err())
		case <-timer.C:
		}
	}

	return res, err
}

// Do is a convenience wrapper around Retry for functions that return only an error.
func Do(attempts int, baseSleep time.Duration, f func() error, opts ...Option) error {
	_, err := Retry(attempts, baseSleep, func() (struct{}, error) {
		return struct{}{}, f()
	}, opts...)
	return err
}

// DoContext is a convenience wrapper around RetryContext for functions that return
// only an error.
func DoContext(ctx context.Context, attempts int, baseSleep time.Duration, f func(context.Context) error, opts ...Option) error {
	_, err := RetryContext(ctx, attempts, baseSleep, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, f(ctx)
	}, opts...)
	return err
}

// backoff returns the jittered sleep before the (i+1)-th retry. The nominal
// backoff is baseSleep * 2^i, optionally capped at maxSleep, and the returned
// duration is drawn uniformly from [0, nominal] (AWS full-jitter strategy).
func backoff(baseSleep time.Duration, i int, maxSleep time.Duration) time.Duration {
	if baseSleep <= 0 || i < 0 {
		return 0
	}

	// Determine the maximum shift that keeps baseSleep * 2^shift from overflowing
	// time.Duration (an int64 nanosecond count). For baseSleep > 0, uint64 has at
	// least one leading zero, so this subtraction never underflows.
	maxShift := uint(bits.LeadingZeros64(uint64(baseSleep))) - 1
	shift := uint(i)
	if shift > maxShift {
		shift = maxShift
	}

	nominal := baseSleep << shift
	if maxSleep > 0 && nominal > maxSleep {
		nominal = maxSleep
	}
	if nominal <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(nominal) + 1))
}

// NonRetryableError signals to Retry that the wrapped error should not trigger a
// further retry. Use NewNonRetryableError to construct one.
type NonRetryableError struct {
	error
}

// Unwrap returns the wrapped error so that errors.Is and errors.As traverse the chain.
func (e NonRetryableError) Unwrap() error {
	return e.error
}

// NewNonRetryableError wraps err so that Retry stops immediately instead of retrying.
func NewNonRetryableError(err error) NonRetryableError {
	return NonRetryableError{err}
}
