package evals

import (
	"errors"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/verdict"
)

func TestT_Check_recordsPassAndFail(t *testing.T) {
	t.Parallel()
	rec := newT()

	if ok := rec.Check("ok", true); !ok {
		t.Fatal("Check(true) returned false")
	}
	if ok := rec.Check("bad", false); ok {
		t.Fatal("Check(false) returned true")
	}
	checks, status := rec.checksAndStatus()
	if len(checks) != 2 {
		t.Fatalf("checks = %d, want 2", len(checks))
	}
	if checks[0].Status != evalspb.Status_PASSED || checks[1].Status != evalspb.Status_FAILED {
		t.Fatalf("statuses = %v, %v", checks[0].Status, checks[1].Status)
	}
	if status != evalspb.Status_FAILED {
		t.Fatalf("rolled-up status = %v, want FAILED", status)
	}
}

func TestT_Checkf_message(t *testing.T) {
	t.Parallel()
	rec := newT()
	rec.Checkf("id", false, "got %d, want %d", 3, 4)
	checks, _ := rec.checksAndStatus()
	if checks[0].Message != "got 3, want 4" {
		t.Fatalf("message = %q", checks[0].Message)
	}
}

func TestT_NoErr(t *testing.T) {
	t.Parallel()
	rec := newT()
	if !rec.NoErr("ok", nil) {
		t.Fatal("NoErr(nil) returned false")
	}
	if rec.NoErr("bad", errors.New("boom")) {
		t.Fatal("NoErr(err) returned true")
	}
	checks, _ := rec.checksAndStatus()
	if checks[1].Message != "boom" {
		t.Fatalf("message = %q, want boom", checks[1].Message)
	}
}

func TestT_Max(t *testing.T) {
	t.Parallel()
	rec := newT()
	if !rec.Max("under", 10*time.Millisecond, 100*time.Millisecond) {
		t.Fatal("Max under-limit returned false")
	}
	if rec.Max("over", 200*time.Millisecond, 100*time.Millisecond) {
		t.Fatal("Max over-limit returned true")
	}
}

func TestT_Score(t *testing.T) {
	t.Parallel()
	rec := newT()
	if !rec.Score("quality", 0.9, 0.5, "great") {
		t.Fatal("Score above threshold returned false")
	}
	if rec.Score("safety", 0.2, 0.5, "") {
		t.Fatal("Score below threshold returned true")
	}
	metrics, status := rec.metricsAndStatus()
	if len(metrics) != 2 {
		t.Fatalf("metrics = %d, want 2", len(metrics))
	}
	if metrics[0].Score == nil || *metrics[0].Score != 0.9 {
		t.Fatalf("score = %v, want 0.9", metrics[0].Score)
	}
	if metrics[0].Message != "great" {
		t.Fatalf("rationale message = %q, want %q", metrics[0].Message, "great")
	}
	if metrics[1].Message == "" {
		t.Fatal("failed Score with empty rationale should synthesize a message")
	}
	if status != evalspb.Status_FAILED {
		t.Fatalf("rolled-up status = %v", status)
	}
}

func TestT_ReservedUserCheckIDRejected(t *testing.T) {
	t.Parallel()
	rec := newT()
	if rec.Check("_evals.mine", true) {
		t.Fatal("reserved user check id returned true")
	}
	checks, status := rec.checksAndStatus()
	if len(checks) != 1 {
		t.Fatalf("checks = %d, want 1 reserved marker", len(checks))
	}
	if checks[0].ID != verdict.IDReservedCheckID || checks[0].Status != evalspb.Status_FAILED {
		t.Fatalf("reserved marker = %+v", checks[0])
	}
	if status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED", status)
	}
}

func TestT_FrameworkCheckIDRejected(t *testing.T) {
	t.Parallel()
	rec := newT()
	if rec.Check(verdict.IDTeardown, true) {
		t.Fatal("framework check id should be reserved for internal emitters")
	}
	checks, _ := rec.checksAndStatus()
	if len(checks) != 1 || checks[0].ID != verdict.IDReservedCheckID {
		t.Fatalf("checks = %+v", checks)
	}
}

func TestT_DuplicateID(t *testing.T) {
	t.Parallel()
	rec := newT()
	if !rec.Check("only", true) {
		t.Fatal("first Check returned false")
	}
	if rec.Check("only", true) {
		t.Fatal("duplicate Check returned true; must fail-loud")
	}
	checks, status := rec.checksAndStatus()
	if len(checks) != 2 {
		t.Fatalf("checks = %d, want 2 (original + duplicate marker)", len(checks))
	}
	if checks[1].ID != DuplicateCheckIDName || checks[1].Status != evalspb.Status_FAILED {
		t.Fatalf("duplicate marker = %+v", checks[1])
	}
	if status != evalspb.Status_FAILED {
		t.Fatalf("status = %v, want FAILED because of duplicate marker", status)
	}

	rec.Check("only", false)
	checks2, _ := rec.checksAndStatus()
	if len(checks2) != 2 {
		t.Fatalf("checks after second duplicate = %d, want still 2 (single marker)", len(checks2))
	}
}

func TestT_emptyFailsRollup(t *testing.T) {
	t.Parallel()
	rec := newT()
	checks, status := rec.checksAndStatus()
	if len(checks) != 1 {
		t.Fatalf("checks = %v, want synthetic no-checks leaf", checks)
	}
	if checks[0].ID != verdict.IDNoChecksRecorded {
		t.Fatalf("check id = %q, want %q", checks[0].ID, verdict.IDNoChecksRecorded)
	}
	if status != evalspb.Status_FAILED {
		t.Fatalf("empty status = %v, want FAILED", status)
	}
}

func TestT_PassEscapeHatch(t *testing.T) {
	t.Parallel()
	rec := newT()
	if !rec.Pass("smoke") {
		t.Fatal("Pass returned false")
	}
	checks, status := rec.checksAndStatus()
	if status != evalspb.Status_PASSED {
		t.Fatalf("status = %v, want PASSED", status)
	}
	if len(checks) != 1 || checks[0].ID != "smoke" {
		t.Fatalf("checks = %+v", checks)
	}
}
