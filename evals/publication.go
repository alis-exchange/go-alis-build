package evals

import (
	"context"
	"errors"
	"io"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/report"
	pubsubreport "go.alis.build/evals/report/pubsub"
)

const defaultSuitePublishTimeout = 10 * time.Second

var newStandardReporter = func(ctx context.Context) (report.Reporter, error) {
	return pubsubreport.New(ctx)
}

// publishRun deliberately uses a fresh bounded context. RunAndPublish must
// publish partial results after the execution context is cancelled.
func publishRun(_ context.Context, cfg runConfig, run *evalspb.Run) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultSuitePublishTimeout)
	defer cancel()

	reporter := cfg.reporter
	owned := false
	if reporter == nil {
		var err error
		reporter, err = newStandardReporter(ctx)
		if err != nil {
			return err
		}
		owned = true
	}
	var joined error
	if reporter != nil {
		joined = errors.Join(joined, reporter.ReportRun(ctx, run))
	}
	if owned {
		if closer, ok := reporter.(io.Closer); ok {
			joined = errors.Join(joined, closer.Close())
		}
	}
	return joined
}
