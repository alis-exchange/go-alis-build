package report

import (
	"context"
	"errors"
	"testing"

	evalspb "go.alis.build/common/alis/evals/v1"
)

type recordingReporter struct {
	names []string
}

func (r *recordingReporter) ReportRun(_ context.Context, run *evalspb.Run) error {
	r.names = append(r.names, run.GetName())
	return nil
}

type failingReporter struct{}

func (failingReporter) ReportRun(context.Context, *evalspb.Run) error {
	return errors.New("sink unavailable")
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
