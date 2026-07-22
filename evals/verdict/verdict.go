// Package verdict centralizes pass/fail rollup policy for evals suite kinds.
package verdict

import evalspb "go.alis.build/common/alis/evals/v1"

// Leaf is one recorded check or metric used for case-level rollup.
type Leaf struct {
	ID      string
	Status  evalspb.Status
	Message string
}

// Evidence is the inputs Case rollup considers besides policy defaults.
type Evidence struct {
	Leaves    []Leaf
	Attempted bool
	Errors    int64
}

// Policy configures how Evidence rolls up to a case, suite, or run status.
type Policy struct {
	RequireEvidence              bool
	FailOnUnrepresentedErrors    bool
	RequireSuccessfulObservation bool
	NotEvaluatedRollsUpAs        evalspb.Status
}

// Case rolls up leaf evidence into a single case status.
func Case(e Evidence, p Policy) (evalspb.Status, []Leaf) {
	if p.RequireEvidence && len(e.Leaves) == 0 {
		return evalspb.Status_FAILED, nil
	}
	if p.FailOnUnrepresentedErrors && e.Errors > 0 {
		return evalspb.Status_FAILED, e.Leaves
	}
	for _, l := range e.Leaves {
		if l.Status != evalspb.Status_PASSED {
			return evalspb.Status_FAILED, e.Leaves
		}
	}
	if p.RequireSuccessfulObservation && len(e.Leaves) == 0 {
		return evalspb.Status_FAILED, e.Leaves
	}
	return evalspb.Status_PASSED, e.Leaves
}

// Suite aggregates per-case statuses within one suite execution.
func Suite(statuses []evalspb.Status, p Policy) evalspb.Status {
	return rollup(statuses, p)
}

// Run aggregates per-case statuses across an entire run.
func Run(statuses []evalspb.Status, p Policy) evalspb.Status {
	return rollup(statuses, p)
}

func rollup(statuses []evalspb.Status, p Policy) evalspb.Status {
	if len(statuses) == 0 {
		return evalspb.Status_PASSED
	}

	neutral := p.NotEvaluatedRollsUpAs
	if neutral == evalspb.Status_STATUS_UNSPECIFIED {
		neutral = evalspb.Status_FAILED
	}

	hasFailed := false
	allNotEvaluated := true
	for _, s := range statuses {
		switch s {
		case evalspb.Status_FAILED:
			hasFailed = true
			allNotEvaluated = false
		case evalspb.Status_PASSED:
			allNotEvaluated = false
		case evalspb.Status_NOT_EVALUATED:
			continue
		default:
			if s != neutral {
				hasFailed = true
			}
			allNotEvaluated = false
		}
	}
	if hasFailed {
		return evalspb.Status_FAILED
	}
	if allNotEvaluated {
		return evalspb.Status_NOT_EVALUATED
	}
	return evalspb.Status_PASSED
}
