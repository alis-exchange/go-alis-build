package report

import (
	"context"

	evalspb "go.alis.build/common/alis/evals/v1"
)

// Reporter receives a materialized Run as soon as a suite completes.
// Implementations may write to Pub/Sub, BigQuery, a database, or a log.
type Reporter interface {
	ReportRun(ctx context.Context, run *evalspb.Run) error
}

// NoOpReporter satisfies the hook without I/O.
type NoOpReporter struct{}

func (NoOpReporter) ReportRun(context.Context, *evalspb.Run) error { return nil }

// MultiReporter fans out to multiple reporters.
type MultiReporter []Reporter

func (m MultiReporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
	for _, r := range m {
		if err := r.ReportRun(ctx, run); err != nil {
			return err
		}
	}
	return nil
}
