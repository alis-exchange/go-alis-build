package report

import (
	"context"
	"errors"

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

// FailFast fans out to multiple reporters in order and returns on the first error.
type FailFast []Reporter

// ReportRun invokes each non-nil reporter in order; the first error stops fan-out.
func (m FailFast) ReportRun(ctx context.Context, run *evalspb.Run) error {
	for _, r := range m {
		if r == nil {
			continue
		}
		if err := r.ReportRun(ctx, run); err != nil {
			return err
		}
	}
	return nil
}

// All fans out to every non-nil reporter and joins all errors with [errors.Join].
// Reporters run serially; worst-case latency is the sum of each sink's per-call
// timeout (bundled non-log reporters default to 10s each).
type All []Reporter

// ReportRun invokes every non-nil reporter even when an earlier one fails.
func (m All) ReportRun(ctx context.Context, run *evalspb.Run) error {
	var joined error
	for _, r := range m {
		if r == nil {
			continue
		}
		if err := r.ReportRun(ctx, run); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// MultiReporter is an alias for [FailFast] preserving the original fail-fast
// fan-out semantics. Prefer [All] when every sink should be attempted.
type MultiReporter = FailFast
