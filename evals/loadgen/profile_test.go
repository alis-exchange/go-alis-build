package loadgen

import (
	"errors"
	"testing"
	"time"
)

func TestProfile_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		p       Profile
		wantErr bool
	}{
		{"valid", Profile{QPS: 5, Concurrency: 2, Duration: time.Second}, false},
		{"valid warmup", Profile{QPS: 5, Concurrency: 2, Duration: time.Second, Warmup: 100 * time.Millisecond}, false},
		{"zero qps", Profile{QPS: 0, Concurrency: 2, Duration: time.Second}, true},
		{"negative qps", Profile{QPS: -1, Concurrency: 2, Duration: time.Second}, true},
		{"zero concurrency", Profile{QPS: 5, Concurrency: 0, Duration: time.Second}, true},
		{"zero duration", Profile{QPS: 5, Concurrency: 2, Duration: 0}, true},
		{"negative warmup", Profile{QPS: 5, Concurrency: 2, Duration: time.Second, Warmup: -1}, true},
		{"negative timeout", Profile{QPS: 5, Concurrency: 2, Duration: time.Second, RequestTimeout: -1}, true},
		{
			"valid qps stages",
			Profile{
				Concurrency: 2,
				Duration:    200 * time.Millisecond,
				QPSStages: []Stage{
					{Duration: 100 * time.Millisecond, Target: 10},
					{Duration: 100 * time.Millisecond, Target: 20},
				},
			},
			false,
		},
		{
			"qps stages duration mismatch",
			Profile{
				Concurrency: 2,
				Duration:    200 * time.Millisecond,
				QPSStages:   []Stage{{Duration: 100 * time.Millisecond, Target: 10}},
			},
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.p.validate()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, ErrInvalidProfile{}) {
					t.Fatalf("err = %v, want ErrInvalidProfile", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestProfile_ResolvedRequestTimeout(t *testing.T) {
	t.Parallel()

	if got := (Profile{}).resolvedRequestTimeout(); got != defaultRequestTimeout {
		t.Fatalf("default = %v, want %v", got, defaultRequestTimeout)
	}
	custom := Profile{RequestTimeout: 5 * time.Second}
	if got := custom.resolvedRequestTimeout(); got != 5*time.Second {
		t.Fatalf("custom = %v, want 5s", got)
	}
}
