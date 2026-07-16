package evals

import (
	"fmt"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/execution"
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
	DuplicateCheckIDName = "duplicate-check-id"
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

// claim reserves id for this case. Returns false and records one duplicate-check
// leaf when id was already used in the same case.
func (t *T) claim(id string) bool {
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
		return nil, evalspb.Status_PASSED
	}
	out := make([]execution.Check, 0, len(t.leaves))
	status := evalspb.Status_PASSED
	for _, l := range t.leaves {
		out = append(out, execution.Check{ID: l.id, Status: l.status, Message: l.message})
		if l.status != evalspb.Status_PASSED {
			status = evalspb.Status_FAILED
		}
	}
	return out, status
}

// metricsAndStatus returns the recorded leaves as execution.Metric values plus
// the rolled-up status for an eval case.
func (t *T) metricsAndStatus() ([]execution.Metric, evalspb.Status) {
	if t == nil || len(t.leaves) == 0 {
		return nil, evalspb.Status_PASSED
	}
	out := make([]execution.Metric, 0, len(t.leaves))
	status := evalspb.Status_PASSED
	for _, l := range t.leaves {
		m := execution.Metric{ID: l.id, Status: l.status, Message: l.message, Threshold: l.threshold}
		if l.score != nil {
			s := *l.score
			m.Score = new(s)
		}
		out = append(out, m)
		if l.status != evalspb.Status_PASSED {
			status = evalspb.Status_FAILED
		}
	}
	return out, status
}
