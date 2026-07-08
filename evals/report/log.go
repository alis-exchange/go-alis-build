package report

import (
	"context"
	"strconv"
	"strings"

	"go.alis.build/alog"
	evalspb "go.alis.build/common/alis/evals/v1"
)

// LogReporter writes a one-line summary of each completed Run to alog. It is
// the default reporter for local development and any deployment that has no
// external sink (Pub/Sub, BigQuery, Spanner, etc.) wired up. Passing runs are
// logged at Info; failing runs at Warn so they stand out in Cloud Logging.
type LogReporter struct{}

// ReportRun implements Reporter.
func (LogReporter) ReportRun(ctx context.Context, run *evalspb.Run) error {
	if run == nil {
		return nil
	}
	summary := formatRun(run)
	if run.GetStatus() == evalspb.Status_PASSED {
		alog.Infof(ctx, "evals run: %s", summary)
	} else {
		alog.Warnf(ctx, "evals run: %s", summary)
	}
	return nil
}

func formatRun(run *evalspb.Run) string {
	var b strings.Builder
	b.WriteString(run.GetName())
	b.WriteString(" type=")
	b.WriteString(strings.TrimPrefix(run.GetType().String(), "TYPE_"))
	b.WriteString(" status=")
	b.WriteString(run.GetStatus().String())

	total, passed, failed, skipped := runCounts(run)
	if total > 0 {
		b.WriteString(" cases=")
		b.WriteString(strconv.Itoa(total))
		b.WriteString(" passed=")
		b.WriteString(strconv.Itoa(passed))
		if failed > 0 {
			b.WriteString(" failed=")
			b.WriteString(strconv.Itoa(failed))
		}
		if skipped > 0 {
			b.WriteString(" skipped=")
			b.WriteString(strconv.Itoa(skipped))
		}
	}

	if start, end := run.GetStartTime(), run.GetEndTime(); start != nil && end != nil {
		if d := end.AsTime().Sub(start.AsTime()); d > 0 {
			b.WriteString(" duration=")
			b.WriteString(d.String())
		}
	}
	if op := run.GetOperation(); op != "" {
		b.WriteString(" op=")
		b.WriteString(op)
	}
	if bid := run.GetBatchId(); bid != "" {
		b.WriteString(" batch=")
		b.WriteString(bid)
	}
	return b.String()
}

func runCounts(run *evalspb.Run) (total, passed, failed, skipped int) {
	switch d := run.GetData().(type) {
	case *evalspb.Run_IntegrationTest:
		for _, c := range d.IntegrationTest.GetCases() {
			total++
			bumpCounts(c.GetStatus(), &passed, &failed, &skipped)
		}
	case *evalspb.Run_AgentEval:
		for _, c := range d.AgentEval.GetCases() {
			total++
			bumpCounts(c.GetStatus(), &passed, &failed, &skipped)
		}
	case *evalspb.Run_LoadTest:
		for _, c := range d.LoadTest.GetCases() {
			total++
			bumpCounts(c.GetStatus(), &passed, &failed, &skipped)
		}
	}
	return
}

func bumpCounts(status evalspb.Status, passed, failed, skipped *int) {
	switch status {
	case evalspb.Status_PASSED:
		*passed++
	case evalspb.Status_NOT_EVALUATED:
		*skipped++
	default:
		*failed++
	}
}
