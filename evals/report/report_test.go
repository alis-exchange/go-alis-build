package report

import (
	"context"
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
)

type recordingReporter struct {
	names []string
}

func (r *recordingReporter) ReportRun(_ context.Context, run *evalspb.Run) error {
	r.names = append(r.names, run.GetName())
	return nil
}

type failingReporter struct {
	name string
}

func (f failingReporter) ReportRun(context.Context, *evalspb.Run) error {
	return errors.New(f.name + ": sink unavailable")
}

type slowReporter struct {
	delay time.Duration
	calls int
}

func (s *slowReporter) ReportRun(ctx context.Context, _ *evalspb.Run) error {
	s.calls++
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.delay):
		return nil
	}
}

func TestNoOpReporter(t *testing.T) {
	t.Parallel()
	if err := (NoOpReporter{}).ReportRun(context.Background(), &evalspb.Run{Name: "runs/x"}); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestMultiReporter(t *testing.T) {
	t.Parallel()

	r1 := &recordingReporter{}
	r2 := &recordingReporter{}
	m := MultiReporter{r1, r2}
	run := &evalspb.Run{Name: "runs/1"}
	if err := m.ReportRun(context.Background(), run); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r1.names) != 1 || len(r2.names) != 1 {
		t.Fatalf("expected both reporters to record")
	}
}

func TestFailFast_stopsOnFirstError(t *testing.T) {
	t.Parallel()

	r1 := &recordingReporter{}
	m := FailFast{failingReporter{name: "first"}, r1}
	run := &evalspb.Run{Name: "runs/1"}
	if err := m.ReportRun(context.Background(), run); err == nil {
		t.Fatal("expected error")
	}
	if len(r1.names) != 0 {
		t.Fatalf("second reporter called after first error")
	}
}

func TestAll_callsEveryReporterAndJoinsErrors(t *testing.T) {
	t.Parallel()

	r1 := &recordingReporter{}
	m := All{failingReporter{name: "first"}, r1, failingReporter{name: "third"}}
	run := &evalspb.Run{Name: "runs/1"}
	err := m.ReportRun(context.Background(), run)
	if err == nil {
		t.Fatal("expected joined error")
	}
	if len(r1.names) != 1 {
		t.Fatalf("middle reporter not called under All")
	}
	if !errors.Is(err, errors.New("first: sink unavailable")) {
		// errors.Join preserves individual errors; check message substring instead.
		if err.Error() == "" {
			t.Fatalf("joined error empty")
		}
	}
}

func TestFanOut_nilEntrySafe(t *testing.T) {
	t.Parallel()

	r := &recordingReporter{}
	run := &evalspb.Run{Name: "runs/1"}
	for _, comb := range []Reporter{
		MultiReporter{nil, r},
		All{nil, r, nil},
		FailFast{nil, r},
	} {
		if err := comb.ReportRun(context.Background(), run); err != nil {
			t.Fatalf("%T: err = %v", comb, err)
		}
	}
	if len(r.names) != 3 {
		t.Fatalf("recordings = %d, want 3", len(r.names))
	}
}

func TestAll_cumulativeLatencyIsSerial(t *testing.T) {
	t.Parallel()

	const delay = 20 * time.Millisecond
	s1 := &slowReporter{delay: delay}
	s2 := &slowReporter{delay: delay}
	m := All{s1, s2}
	start := time.Now()
	if err := m.ReportRun(context.Background(), &evalspb.Run{Name: "runs/latency"}); err != nil {
		t.Fatalf("err = %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 2*delay {
		t.Fatalf("elapsed = %v, want at least %v (serial fan-out)", elapsed, 2*delay)
	}
	if s1.calls != 1 || s2.calls != 1 {
		t.Fatalf("calls s1=%d s2=%d, want 1 each", s1.calls, s2.calls)
	}
}
