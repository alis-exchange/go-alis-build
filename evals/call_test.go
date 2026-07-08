package evals

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCall_success(t *testing.T) {
	t.Parallel()
	got := Call(context.Background(), func(ctx context.Context) (string, error) {
		time.Sleep(2 * time.Millisecond)
		return "ok", nil
	})
	if got.Resp != "ok" {
		t.Fatalf("resp = %q", got.Resp)
	}
	if got.Err != nil {
		t.Fatalf("err = %v", got.Err)
	}
	if got.Latency <= 0 {
		t.Fatalf("latency = %v, want positive", got.Latency)
	}
}

func TestCall_preservesPartialResponse(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("boom")
	got := Call(context.Background(), func(ctx context.Context) (int, error) {
		return 42, wantErr
	})
	if got.Resp != 42 {
		t.Fatalf("resp = %d, want 42 (partial response preserved on error)", got.Resp)
	}
	if !errors.Is(got.Err, wantErr) {
		t.Fatalf("err = %v", got.Err)
	}
}

func TestCall_propagatesContext(t *testing.T) {
	t.Parallel()
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "signal")
	Call(ctx, func(ctx context.Context) (int, error) {
		if v, _ := ctx.Value(key{}).(string); v != "signal" {
			t.Fatalf("ctx value = %q", v)
		}
		return 0, nil
	})
}
