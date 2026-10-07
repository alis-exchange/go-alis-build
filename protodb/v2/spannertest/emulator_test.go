package spannertest

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// TestResolve pins the three rules: a set SPANNER_EMULATOR_HOST wins, else
// SPANNERTEST_EMULATOR asks for a container, else skip with a message
// naming both variables.
func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		wantHost  string
		wantStart bool
		wantSkip  bool
	}{
		{
			name:     "reuse wins over start",
			env:      map[string]string{"SPANNER_EMULATOR_HOST": "localhost:9010", "SPANNERTEST_EMULATOR": "1"},
			wantHost: "localhost:9010",
		},
		{
			name:      "start",
			env:       map[string]string{"SPANNERTEST_EMULATOR": "1"},
			wantStart: true,
		},
		{
			name:     "skip",
			env:      map[string]string{},
			wantSkip: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, start, skip := resolve(func(k string) string { return tt.env[k] })
			if host != tt.wantHost || start != tt.wantStart {
				t.Fatalf("resolve = (%q, %v), want (%q, %v)", host, start, tt.wantHost, tt.wantStart)
			}
			if (skip != "") != tt.wantSkip {
				t.Fatalf("skip = %q, want skip=%v", skip, tt.wantSkip)
			}
			if tt.wantSkip && (!strings.Contains(skip, "SPANNER_EMULATOR_HOST") || !strings.Contains(skip, "SPANNERTEST_EMULATOR")) {
				t.Errorf("skip message %q must name both variables", skip)
			}
		})
	}
}

// TestDefaultImagePinned pins the emulator version the probe validated, so
// a bump is a deliberate change with its own test run.
func TestDefaultImagePinned(t *testing.T) {
	if want := "gcr.io/cloud-spanner-emulator/emulator:1.5.57"; DefaultImage != want {
		t.Errorf("DefaultImage = %q, want %q", DefaultImage, want)
	}
}

// skipRecorder captures the first Skip and stops the goroutine, so a test
// can assert a helper skipped without skipping itself.
type skipRecorder struct {
	testing.TB
	msg string
}

// Skip records args and exits the calling goroutine.
func (r *skipRecorder) Skip(args ...any) { r.msg = fmt.Sprint(args...); runtime.Goexit() }

// Skipf records the message and exits the calling goroutine.
func (r *skipRecorder) Skipf(format string, args ...any) {
	r.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// recordSkip runs call against a skipRecorder on its own goroutine and
// returns the skip message, or "" if call returned without skipping.
func recordSkip(t *testing.T, call func(testing.TB)) string {
	t.Helper()
	rec := &skipRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		call(rec)
	}()
	<-done
	return rec.msg
}

// TestHostSkipsWithoutEmulator pins AC5 for Host: with neither variable
// set it skips with a message naming both. Runs without Docker.
func TestHostSkipsWithoutEmulator(t *testing.T) {
	t.Setenv("SPANNER_EMULATOR_HOST", "")
	t.Setenv("SPANNERTEST_EMULATOR", "")
	msg := recordSkip(t, func(tb testing.TB) { Host(tb) })
	if !strings.Contains(msg, "SPANNER_EMULATOR_HOST") || !strings.Contains(msg, "SPANNERTEST_EMULATOR") {
		t.Errorf("Host skip message %q must name both variables", msg)
	}
}
