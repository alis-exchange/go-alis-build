package adk

import (
	"errors"
	"fmt"
	"strings"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	evalspb "go.alis.build/common/alis/evals"
)

// SuiteCaseName returns a suite case name for the raw ADK case id by
// replacing every "." with "_". ADK case ids may contain ".", but evals
// suite case names may not, because the run qualifies case ids as
// "{suite}.{case}".
//
// Two ids that differ only by "." versus "_" map to the same name. The suite
// then reports evals.ErrDuplicateCase from Run before any case starts.
func SuiteCaseName(id string) string {
	return strings.ReplaceAll(id, ".", "_")
}

// CaseRecorder receives one ADK case result. *evals.AgentEvalResult
// satisfies it, so an AgentEvalSuite case can pass its builder to
// [ProviderCase.RecordTo]. It is declared here rather than imported so this
// package does not depend on the root evals package and its Pub/Sub reporter.
type CaseRecorder interface {
	// SetSessionID records the ADK session id.
	SetSessionID(id string)
	// AddMetric appends one metric result.
	AddMetric(m *evalspb.AgentEvalResults_Case_Metric)
	// SetJudgeInfo declares the case's judge provenance and call counts.
	SetJudgeInfo(j *evalspb.AgentEvalResults_JudgeInfo)
	// Fail marks the case failed.
	Fail(err error)
	// SetNotEvaluated marks the case NOT_EVALUATED while keeping its data;
	// a failure still wins.
	SetNotEvaluated(reason string)
}

// errFinalStatusFailed is recorded when ADK reports the case FAILED.
var errFinalStatusFailed = errors.New("adk: final eval status FAILED")

// notEvaluatedReason is the reason passed to SetNotEvaluated when ADK
// reports the case NOT_EVALUATED.
const notEvaluatedReason = "adk: final eval status NOT_EVALUATED"

// RecordTo records the case into r: the session id when set, one metric per
// ADK metric result, the judge info when [ProviderCase.Judge] is non-zero,
// then a failure when ADK's final status is FAILED, or a NOT_EVALUATED mark
// for any final status other than PASSED and FAILED: NOT_EVALUATED, unset
// (zero), or a value this package does not know.
//
// r must be a usable recorder. A nil interface does nothing, but a typed nil
// pointer such as a nil *evals.AgentEvalResult is not detected and panics.
//
// The suite derives case status from what is recorded. ADK FAILED gives a
// FAILED suite case. Any status other than PASSED and FAILED gives a
// NOT_EVALUATED suite case that keeps its session, metrics and judge, unless
// a metric FAILED, which wins; it never reports PASSED. ADK PASSED with a
// FAILED metric gives FAILED.
func (c ProviderCase) RecordTo(r CaseRecorder) {
	if r == nil {
		return
	}
	if c.Result.SessionID != "" {
		r.SetSessionID(c.Result.SessionID)
	}
	for _, mr := range c.Result.OverallEvalMetricResults {
		r.AddMetric(metricFromADK(mr))
	}
	if j := judgeInfo(c.Judge); j != nil {
		r.SetJudgeInfo(j)
	}
	switch status := c.Result.FinalEvalStatus; status {
	case models.EvalStatusPassed:
	case models.EvalStatusFailed:
		r.Fail(errFinalStatusFailed)
	case models.EvalStatusNotEvaluated:
		r.SetNotEvaluated(notEvaluatedReason)
	default:
		// models.EvalStatus is a plain int without a String method, so the
		// number is the clearest name for an unset or unknown value.
		r.SetNotEvaluated(fmt.Sprintf("adk: unknown final eval status %d", int(status)))
	}
}
