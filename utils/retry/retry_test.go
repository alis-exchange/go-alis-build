package retry

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_SucceedsOnFirstTry(t *testing.T) {
	calls := 0
	res, err := Retry(3, time.Millisecond, func() (int, error) {
		calls++
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != 42 {
		t.Errorf("res = %d, want 42", res)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetry_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	res, err := Retry(5, time.Microsecond, func() (string, error) {
		calls++
		if calls < 3 {
			return "", errors.New("transient")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "ok" {
		t.Errorf("res = %q, want ok", res)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetry_ExhaustsAllAttempts(t *testing.T) {
	calls := 0
	sentinel := errors.New("boom")
	_, err := Retry(3, time.Microsecond, func() (any, error) {
		calls++
		return nil, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want %v", err, sentinel)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetry_NonRetryableStopsImmediately(t *testing.T) {
	calls := 0
	sentinel := errors.New("stop")
	_, err := Retry(5, time.Microsecond, func() (any, error) {
		calls++
		return nil, NewNonRetryableError(sentinel)
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want %v", err, sentinel)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetry_NonRetryableWrappedByFmtErrorf(t *testing.T) {
	calls := 0
	sentinel := errors.New("stop")
	_, err := Retry(5, time.Microsecond, func() (any, error) {
		calls++
		return nil, fmt.Errorf("context: %w", NewNonRetryableError(sentinel))
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want %v", err, sentinel)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (NonRetryableError should short-circuit even when wrapped)", calls)
	}
}

func TestRetry_ZeroAttemptsStillInvokesOnce(t *testing.T) {
	calls := 0
	sentinel := errors.New("x")
	_, err := Retry(0, time.Microsecond, func() (any, error) {
		calls++
		return nil, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want %v", err, sentinel)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetry_OnRetryCallback(t *testing.T) {
	var (
		attempts []int
		errs     []error
		sleeps   []time.Duration
	)
	sentinel := errors.New("boom")

	_, err := Retry(4, 100*time.Microsecond, func() (any, error) {
		return nil, sentinel
	},
		WithMaxSleep(500*time.Microsecond),
		WithOnRetry(func(attempt int, err error, nextSleep time.Duration) {
			attempts = append(attempts, attempt)
			errs = append(errs, err)
			sleeps = append(sleeps, nextSleep)
		}),
	)

	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
	// 4 attempts => 3 sleeps => 3 OnRetry invocations.
	if len(attempts) != 3 {
		t.Fatalf("OnRetry called %d times, want 3", len(attempts))
	}
	for i, a := range attempts {
		if a != i+1 {
			t.Errorf("attempts[%d] = %d, want %d", i, a, i+1)
		}
		if !errors.Is(errs[i], sentinel) {
			t.Errorf("errs[%d] = %v, want %v", i, errs[i], sentinel)
		}
		if sleeps[i] < 0 || sleeps[i] > 500*time.Microsecond {
			t.Errorf("sleeps[%d] = %v, want within [0, 500us]", i, sleeps[i])
		}
	}
}

func TestRetry_WithMaxSleepCapsBackoff(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive test skipped in short mode")
	}

	base := 10 * time.Millisecond
	maxSleep := 15 * time.Millisecond

	start := time.Now()
	_, err := Retry(6, base, func() (any, error) {
		return nil, errors.New("keep failing")
	}, WithMaxSleep(maxSleep))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	// 6 attempts => 5 sleeps, each capped at 15ms (jittered down to [0, 15ms]).
	// Upper bound: 5 * 15ms = 75ms, plus a little overhead. Without a cap it
	// would be 10 + 20 + 40 + 80 + 160 = 310ms, so the cap must be effective.
	if elapsed > 120*time.Millisecond {
		t.Errorf("elapsed = %v, want <= 120ms with MaxSleep cap", elapsed)
	}
}

func TestRetryContext_CancelDuringSleepAborts(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive test skipped in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	var calls atomic.Int32
	sentinel := errors.New("boom")

	start := time.Now()
	_, err := RetryContext(ctx, 10, 100*time.Millisecond, func(ctx context.Context) (any, error) {
		calls.Add(1)
		return nil, sentinel
	})
	elapsed := time.Since(start)

	if !errors.Is(err, sentinel) {
		t.Errorf("err should wrap the last f error (%v); got %v", sentinel, err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err should wrap context.DeadlineExceeded so callers can distinguish cancellation from exhaustion; got %v", err)
	}
	// Should abort well before 10 * 100ms = 1s.
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed = %v, want < 200ms (context should cancel sleep)", elapsed)
	}
	if got := calls.Load(); got > 3 {
		t.Errorf("calls = %d, want few (context should abort early)", got)
	}
}

func TestRetryContext_PassesContextToF(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "hello")

	var got string
	_, err := RetryContext(ctx, 1, time.Microsecond, func(ctx context.Context) (any, error) {
		if v, ok := ctx.Value(ctxKey{}).(string); ok {
			got = v
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("ctx value = %q, want %q", got, "hello")
	}
}

func TestDo_WrapsErrorOnlyFunction(t *testing.T) {
	calls := 0
	sentinel := errors.New("x")
	err := Do(3, time.Microsecond, func() error {
		calls++
		if calls < 2 {
			return sentinel
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestDoContext_RespectsCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive test skipped in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	sentinel := errors.New("x")
	start := time.Now()
	err := DoContext(ctx, 10, 100*time.Millisecond, func(ctx context.Context) error {
		return sentinel
	})
	elapsed := time.Since(start)

	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want %v", err, sentinel)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed = %v, want < 200ms", elapsed)
	}
}

func TestBackoff_AscendsAndCaps(t *testing.T) {
	base := 10 * time.Millisecond
	// Without a cap: nominal is base * 2^i. Since we return uniform [0, nominal],
	// we can't check exact values, but we can check the upper bound.
	for i := 0; i < 5; i++ {
		wantMax := base * (1 << uint(i))
		got := backoff(base, i, 0)
		if got < 0 || got > wantMax {
			t.Errorf("backoff(%v, %d, 0) = %v, want in [0, %v]", base, i, got, wantMax)
		}
	}

	// With a cap: bound is maxSleep regardless of i.
	maxSleep := 25 * time.Millisecond
	for i := 0; i < 10; i++ {
		got := backoff(base, i, maxSleep)
		if got < 0 || got > maxSleep {
			t.Errorf("backoff(%v, %d, %v) = %v, want in [0, %v]", base, i, maxSleep, got, maxSleep)
		}
	}
}

func TestBackoff_OverflowSafeForLargeBase(t *testing.T) {
	// Regression: with the old fixed shift cap of 30, baseSleep values above ~9s
	// could still overflow int64 nanoseconds. The bits.LeadingZeros64-based cap
	// should clamp the shift so nominal never overflows or goes negative.
	bases := []time.Duration{
		10 * time.Second,
		time.Hour,
		24 * time.Hour,
		time.Duration(1) << 60, // near the int64 ceiling
	}
	for _, base := range bases {
		for i := 0; i < 100; i++ {
			got := backoff(base, i, 0)
			if got < 0 {
				t.Errorf("backoff(%v, %d, 0) = %v, want non-negative", base, i, got)
			}
		}
	}
}

func TestBackoff_NegativeIndex(t *testing.T) {
	if got := backoff(time.Second, -1, 0); got != 0 {
		t.Errorf("backoff(1s, -1, 0) = %v, want 0", got)
	}
}

func TestBackoff_HandlesZeroBase(t *testing.T) {
	// Should not panic even when baseSleep is zero.
	for i := 0; i < 5; i++ {
		if got := backoff(0, i, 0); got != 0 {
			t.Errorf("backoff(0, %d, 0) = %v, want 0", i, got)
		}
	}
}

func TestRetry_NonRetryableWithNilInner(t *testing.T) {
	// A caller who does NewNonRetryableError(nil) shouldn't cause Retry to silently
	// return a nil error and mask a signaled failure. Retry falls back to the outer
	// error so the caller can still see the NonRetryableError.
	calls := 0
	_, err := Retry(3, time.Microsecond, func() (any, error) {
		calls++
		return nil, NewNonRetryableError(nil)
	})
	if err == nil {
		t.Fatal("err = nil, want a non-nil NonRetryableError wrapper")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
	var nre NonRetryableError
	if !errors.As(err, &nre) {
		t.Errorf("errors.As should find NonRetryableError; got %v", err)
	}
}

func TestNonRetryableError_ErrorMessage(t *testing.T) {
	inner := errors.New("inner")
	nre := NewNonRetryableError(inner)
	if nre.Error() != "inner" {
		t.Errorf("Error() = %q, want %q", nre.Error(), "inner")
	}
	if !errors.Is(nre, inner) {
		t.Error("errors.Is should find the wrapped error via Unwrap")
	}
}
