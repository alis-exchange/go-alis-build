package evals

import (
	"fmt"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
	"go.alis.build/evals/verdict"
)

// T is the per-case recorder that a CaseFunc receives. Every recording method
// returns whether the leaf passed so authors can guard with a plain return:
//
//	if !t.NoErr("grpc", r.Err) { return }
//
// T is not safe for concurrent use across goroutines within a single case.
type T struct {
	// leaves is the append-only list of recorded assertions for this case.
	leaves []leaf
	// seen tracks ids already claimed; initialized by newT.
	seen map[string]struct{}
	// duplicateHit is set on first duplicate id; suppresses further duplicate leaves.
	duplicateHit bool
	// reservedHit is set on first reserved user id; suppresses further reserved leaves.
	reservedHit bool
}

// leaf is the internal representation of one recorded assertion. Adapters
// translate it to execution.Check (test cases) or execution.Metric (eval
// cases) at result-assembly time.
type leaf struct {
	// id is the user-facing check/metric id from T.Check/Score.
	id string
	// status is PASSED or FAILED for this leaf.
	status evalspb.Status
	// message holds failure detail; empty on pass.
	message string
	// score is set for Score leaves; nil for boolean checks.
	score *float64
	// threshold is the minimum score for Score leaves; zero otherwise.
	threshold float64
}

const (
	// DuplicateCheckIDName is the id used when duplicate check IDs are detected
	// inside one case.
	DuplicateCheckIDName = verdict.IDDuplicateCheckID
)

// newT allocates a per-case recorder with an empty seen set.
func newT() *T {
	return &T{seen: make(map[string]struct{})}
}

// Check records a boolean assertion. Returns whether the assertion passed.
func (t *T) Check(id string, pass bool) bool {
	if !t.claim(id) {
		return false
	}
	return t.append(leaf{id: id, status: statusOf(pass)})
}

// Checkf is Check with a formatted failure message.
func (t *T) Checkf(id string, pass bool, format string, args ...any) bool {
	if !t.claim(id) {
		return false
	}
	msg := ""
	if !pass {
		msg = fmt.Sprintf(format, args...)
	}
	return t.append(leaf{id: id, status: statusOf(pass), message: msg})
}

// NoErr records a pass when err is nil, otherwise a failure carrying err.Error().
func (t *T) NoErr(id string, err error) bool {
	if !t.claim(id) {
		return false
	}
	if err == nil {
		return t.append(leaf{id: id, status: evalspb.Status_PASSED})
	}
	return t.append(leaf{id: id, status: evalspb.Status_FAILED, message: err.Error()})
}

// Max records a pass when got <= limit.
func (t *T) Max(id string, got, limit time.Duration) bool {
	if !t.claim(id) {
		return false
	}
	pass := got <= limit
	msg := ""
	if !pass {
		msg = fmt.Sprintf("%s %s exceeds limit %s", id, got, limit)
	}
	return t.append(leaf{id: id, status: statusOf(pass), message: msg})
}

// Score records a scored leaf that maps to execution.Metric on eval cases.
// Passes (and returns true) when score >= threshold. Rationale is surfaced as
// the metric message.
func (t *T) Score(id string, score, threshold float64, rationale string) bool {
	if !t.claim(id) {
		return false
	}
	pass := score >= threshold
	s := score
	msg := rationale
	if !pass && msg == "" {
		msg = fmt.Sprintf("score %.4f below threshold %.4f", score, threshold)
	}
	return t.append(leaf{
		id:        id,
		status:    statusOf(pass),
		message:   msg,
		score:     &s,
		threshold: threshold,
	})
}

// Pass records an intentional passing leaf for cases that deliberately
// perform no other assertions.
func (t *T) Pass(id string) bool {
	return t.Check(id, true)
}

// RecordSuccess is an alias for [T.Pass].
func (t *T) RecordSuccess(id string) bool {
	return t.Pass(id)
}

// claim reserves id for this case. Returns false and records one duplicate-check
// or reserved-check leaf when id was already used or uses a reserved prefix.
func (t *T) claim(id string) bool {
	if verdict.IsReserved(id) {
		if !t.reservedHit {
			t.reservedHit = true
			t.leaves = append(t.leaves, leaf{
				id:      verdict.IDReservedCheckID,
				status:  evalspb.Status_FAILED,
				message: fmt.Sprintf("reserved check id: %q", id),
			})
		}
		return false
	}
	if _, ok := t.seen[id]; ok {
		if !t.duplicateHit {
			t.duplicateHit = true
			t.leaves = append(t.leaves, leaf{
				id:      DuplicateCheckIDName,
				status:  evalspb.Status_FAILED,
				message: fmt.Sprintf("duplicate check id: %q", id),
			})
		}
		return false
	}
	t.seen[id] = struct{}{}
	return true
}

// append records l and returns whether the leaf passed.
func (t *T) append(l leaf) bool {
	t.leaves = append(t.leaves, l)
	return l.status == evalspb.Status_PASSED
}

// statusOf maps a boolean pass flag to the wire PASSED/FAILED enum.
func statusOf(pass bool) evalspb.Status {
	if pass {
		return evalspb.Status_PASSED
	}
	return evalspb.Status_FAILED
}

// checksAndStatus returns the recorded leaves as execution.Check values plus
// the rolled-up status for a test case.
func (t *T) checksAndStatus() ([]execution.Check, evalspb.Status) {
	if t == nil || len(t.leaves) == 0 {
		status, _ := verdict.Case(verdict.Evidence{}, verdict.IntegrationCasePolicy())
		if status == evalspb.Status_FAILED {
			return []execution.Check{{
				ID:      verdict.IDNoChecksRecorded,
				Status:  evalspb.Status_FAILED,
				Message: "case body recorded no checks or metrics",
			}}, status
		}
		return nil, evalspb.Status_PASSED
	}
	vLeaves := make([]verdict.Leaf, len(t.leaves))
	for i, l := range t.leaves {
		vLeaves[i] = verdict.Leaf{ID: l.id, Status: l.status, Message: l.message}
	}
	status, _ := verdict.Case(verdict.Evidence{Leaves: vLeaves}, verdict.Policy{})
	out := make([]execution.Check, len(t.leaves))
	for i, l := range t.leaves {
		out[i] = execution.Check{ID: l.id, Status: l.status, Message: l.message}
	}
	return out, status
}

// metricsAndStatus returns the recorded leaves as execution.Metric values plus
// the rolled-up status for an eval case.
func (t *T) metricsAndStatus() ([]execution.Metric, evalspb.Status) {
	if t == nil || len(t.leaves) == 0 {
		status, _ := verdict.Case(verdict.Evidence{}, verdict.EvalCasePolicy())
		if status == evalspb.Status_FAILED {
			return []execution.Metric{{
				ID:      verdict.IDNoChecksRecorded,
				Status:  evalspb.Status_FAILED,
				Message: "case body recorded no checks or metrics",
			}}, status
		}
		return nil, evalspb.Status_PASSED
	}
	vLeaves := make([]verdict.Leaf, len(t.leaves))
	for i, l := range t.leaves {
		vLeaves[i] = verdict.Leaf{ID: l.id, Status: l.status, Message: l.message}
	}
	status, _ := verdict.Case(verdict.Evidence{Leaves: vLeaves}, verdict.Policy{})
	out := make([]execution.Metric, len(t.leaves))
	for i, l := range t.leaves {
		m := execution.Metric{ID: l.id, Status: l.status, Message: l.message, Threshold: l.threshold}
		if l.score != nil {
			s := *l.score
			m.Score = new(s)
		}
		out[i] = m
	}
	return out, status
}
