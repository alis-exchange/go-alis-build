package runner

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/evals/internal/result"
)

type stubExecuteCase struct {
	name   string
	status evalspb.Status
	delay  time.Duration
}

type stubExecuteResult struct {
	name   string
	status evalspb.Status
}

func stubExecuteHooks() ExecuteHooks[stubExecuteCase, stubExecuteResult] {
	return ExecuteHooks[stubExecuteCase, stubExecuteResult]{
		CaseName: func(c stubExecuteCase) string { return c.name },
		IsPassed: func(r stubExecuteResult) bool { return r.status == evalspb.Status_PASSED },
		SkippedResult: func(name, reason string) stubExecuteResult {
			return stubExecuteResult{name: name, status: evalspb.Status_NOT_EVALUATED}
		},
		CancelledResult: func(name string) stubExecuteResult {
			return stubExecuteResult{name: name, status: evalspb.Status_NOT_EVALUATED}
		},
	}
}

func runStubCase(_ context.Context, c stubExecuteCase) stubExecuteResult {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	return stubExecuteResult{name: c.name, status: c.status}
}

func TestExecute_stopOnFailureSkipsRemaining(t *testing.T) {
	t.Parallel()

	cases := []stubExecuteCase{
		{name: "first", status: evalspb.Status_PASSED},
		{name: "second", status: evalspb.Status_FAILED},
		{name: "third", status: evalspb.Status_PASSED},
	}

	out, err := Execute(context.Background(), cases, runStubCase, ExecuteOptions{
		StopOnFailure: true,
		Sequential:    true,
	}, stubExecuteHooks())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	if out[0].status != evalspb.Status_PASSED {
		t.Fatalf("out[0].status = %v, want PASSED", out[0].status)
	}
	if out[1].status != evalspb.Status_FAILED {
		t.Fatalf("out[1].status = %v, want FAILED", out[1].status)
	}
	for i := 2; i < 3; i++ {
		if out[i].status != evalspb.Status_NOT_EVALUATED {
			t.Fatalf("out[%d].status = %v, want NOT_EVALUATED", i, out[i].status)
		}
	}
}

func TestExecute_cancelFillsRemainingCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []stubExecuteCase{
		{name: "first", status: evalspb.Status_PASSED},
		{name: "second", status: evalspb.Status_PASSED},
		{name: "third", status: evalspb.Status_PASSED},
	}

	out, err := Execute(ctx, cases, runStubCase, ExecuteOptions{Sequential: true}, stubExecuteHooks())
	if err == nil {
		t.Fatal("Execute: want context error")
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3 cancelled markers", len(out))
	}
	for i, cr := range out {
		if cr.status != evalspb.Status_NOT_EVALUATED {
			t.Fatalf("out[%d].status = %v, want NOT_EVALUATED", i, cr.status)
		}
	}
}

func TestExecute_concurrentRunsAllCases(t *testing.T) {
	t.Parallel()

	const n = 6
	cases := make([]stubExecuteCase, n)
	for i := range cases {
		cases[i] = stubExecuteCase{
			name:   string(rune('a' + i)),
			status: evalspb.Status_PASSED,
			delay:  20 * time.Millisecond,
		}
	}

	start := time.Now()
	out, err := Execute(context.Background(), cases, runStubCase, ExecuteOptions{
		Sequential:  false,
		Concurrency: 4,
	}, stubExecuteHooks())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out) != n {
		t.Fatalf("len(out) = %d, want %d", len(out), n)
	}
	for i, cr := range out {
		if cr.status != evalspb.Status_PASSED {
			t.Fatalf("out[%d].status = %v, want PASSED", i, cr.status)
		}
		if cr.name != cases[i].name {
			t.Fatalf("out[%d].name = %q, want %q (output order preserved)", i, cr.name, cases[i].name)
		}
	}
	if elapsed >= time.Duration(n)*20*time.Millisecond {
		t.Fatalf("elapsed = %v, want concurrent completion faster than sequential", elapsed)
	}
}

func TestExecute_progressFiresPerCompletedCase(t *testing.T) {
	t.Parallel()

	cases := []stubExecuteCase{
		{name: "one", status: evalspb.Status_FAILED},
		{name: "two", status: evalspb.Status_PASSED},
		{name: "three", status: evalspb.Status_PASSED},
	}

	var calls [][2]int
	out, err := Execute(context.Background(), cases, runStubCase, ExecuteOptions{
		StopOnFailure: true,
		Sequential:    true,
		OnCaseComplete: func(completed, total int) {
			calls = append(calls, [2]int{completed, total})
		},
	}, stubExecuteHooks())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	if len(calls) != 3 {
		t.Fatalf("progress calls = %d, want 3", len(calls))
	}
	if calls[2] != [2]int{3, 3} {
		t.Fatalf("final progress = %v, want [3 3]", calls[2])
	}
}

func TestExecute_runCaseMayRecoverPanic(t *testing.T) {
	t.Parallel()

	runWithRecovery := func(ctx context.Context, c stubExecuteCase) (r stubExecuteResult) {
		defer func() {
			if v := recover(); v != nil {
				r = stubExecuteResult{name: c.name, status: evalspb.Status_FAILED}
			}
		}()
		if c.name == "boom" {
			panic("intentional")
		}
		return stubExecuteResult{name: c.name, status: evalspb.Status_PASSED}
	}

	cases := []stubExecuteCase{
		{name: "boom", status: evalspb.Status_PASSED},
		{name: "next", status: evalspb.Status_PASSED},
	}

	out, err := Execute(context.Background(), cases, runWithRecovery, ExecuteOptions{Sequential: true}, stubExecuteHooks())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].status != evalspb.Status_FAILED {
		t.Fatalf("out[0].status = %v, want FAILED", out[0].status)
	}
	if out[1].status != evalspb.Status_PASSED {
		t.Fatalf("out[1].status = %v, want PASSED (batch continues)", out[1].status)
	}
}

func TestExecute_stopOnFailureUsesSkipReason(t *testing.T) {
	t.Parallel()

	hooks := stubExecuteHooks()
	hooks.SkippedResult = func(name, reason string) stubExecuteResult {
		if !strings.Contains(reason, "second") {
			t.Fatalf("skip reason = %q, want mention of failed case", reason)
		}
		return stubExecuteResult{name: name, status: evalspb.Status_NOT_EVALUATED}
	}

	cases := []stubExecuteCase{
		{name: "first", status: evalspb.Status_PASSED},
		{name: "second", status: evalspb.Status_FAILED},
		{name: "third", status: evalspb.Status_PASSED},
	}

	_, err := Execute(context.Background(), cases, runStubCase, ExecuteOptions{
		StopOnFailure: true,
		Sequential:    true,
	}, hooks)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestExecute_cancelMidSuite(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var ran atomic.Int32

	run := func(ctx context.Context, c stubExecuteCase) stubExecuteResult {
		if c.name == "second" {
			cancel()
		}
		ran.Add(1)
		return stubExecuteResult{name: c.name, status: evalspb.Status_PASSED}
	}

	cases := []stubExecuteCase{
		{name: "first", status: evalspb.Status_PASSED},
		{name: "second", status: evalspb.Status_PASSED},
		{name: "third", status: evalspb.Status_PASSED},
	}

	out, err := Execute(ctx, cases, run, ExecuteOptions{Sequential: true}, stubExecuteHooks())
	if err == nil {
		t.Fatal("Execute: want cancel error")
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want partial + cancelled", len(out))
	}
	if out[0].status != evalspb.Status_PASSED {
		t.Fatalf("out[0].status = %v, want PASSED", out[0].status)
	}
	if out[1].status != evalspb.Status_PASSED {
		t.Fatalf("out[1].status = %v, want PASSED (case started before cancel)", out[1].status)
	}
	if out[2].status != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("out[2].status = %v, want NOT_EVALUATED", out[2].status)
	}
}

func TestExecute_concurrentCancellationDoesNotStartQueuedCases(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	var ran atomic.Int32
	run := func(context.Context, stubExecuteCase) stubExecuteResult {
		if ran.Add(1) == 1 {
			close(started)
		}
		<-release
		return stubExecuteResult{name: "started", status: evalspb.Status_PASSED}
	}
	cases := []stubExecuteCase{
		{name: "first"},
		{name: "second"},
		{name: "third"},
	}

	done := make(chan struct{})
	var out []stubExecuteResult
	var err error
	go func() {
		out, err = Execute(ctx, cases, run, ExecuteOptions{Concurrency: 1}, stubExecuteHooks())
		close(done)
	}()
	<-started
	cancel()
	close(release)
	<-done

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute() error = %v, want context.Canceled", err)
	}
	if got := ran.Load(); got != 1 {
		t.Fatalf("runCase calls = %d, want only the case started before cancellation", got)
	}
	if len(out) != len(cases) {
		t.Fatalf("len(out) = %d, want %d", len(out), len(cases))
	}
	notEvaluated := 0
	for _, result := range out {
		if result.status == evalspb.Status_NOT_EVALUATED {
			notEvaluated++
		}
	}
	if notEvaluated != 2 {
		t.Fatalf("NOT_EVALUATED results = %d, want 2", notEvaluated)
	}
}

func TestExecute_integrationSkippedMarker(t *testing.T) {
	t.Parallel()

	hooks := ExecuteHooks[stubExecuteCase, stubExecuteResult]{
		CaseName: func(c stubExecuteCase) string { return c.name },
		IsPassed: func(r stubExecuteResult) bool { return r.status == evalspb.Status_PASSED },
		SkippedResult: func(name, reason string) stubExecuteResult {
			skipped := result.SkippedResult(name, reason)
			return stubExecuteResult{
				name:   skipped.Name,
				status: skipped.Status,
			}
		},
		CancelledResult: func(name string) stubExecuteResult {
			cancelled := result.CancelledCaseResult(name)
			return stubExecuteResult{
				name:   cancelled.Name,
				status: cancelled.Status,
			}
		},
	}

	cases := []stubExecuteCase{
		{name: "first", status: evalspb.Status_FAILED},
		{name: "second", status: evalspb.Status_PASSED},
	}

	out, err := Execute(context.Background(), cases, runStubCase, ExecuteOptions{
		StopOnFailure: true,
		Sequential:    true,
	}, hooks)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out[1].status != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("out[1].status = %v, want NOT_EVALUATED", out[1].status)
	}
}
