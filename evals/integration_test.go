package evals

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
	"go.alis.build/validation"
)

func TestIntegrationSuite_mapsValidatorRulesInDeclarationOrder(t *testing.T) {
	t.Parallel()

	run, err := NewIntegrationSuite("integration-rules").
		AddCase("mixed", func(_ context.Context, v *validation.Validator) {
			v.Custom("first passes", true)
			v.Custom("second fails", false)
			v.Custom("third passes", true)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cases := run.GetIntegrationTest().GetCases()
	if len(cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(cases))
	}
	gotStatuses := []evalspb.Status{}
	gotIDs := []string{}
	gotMessages := []string{}
	for _, check := range cases[0].GetChecks() {
		gotStatuses = append(gotStatuses, check.GetStatus())
		gotIDs = append(gotIDs, check.GetId())
		gotMessages = append(gotMessages, check.GetMessage())
	}
	if !reflect.DeepEqual(gotIDs, []string{"first passes", "second fails", "third passes"}) {
		t.Fatalf("check ids = %v, want declaration order", gotIDs)
	}
	if !reflect.DeepEqual(gotStatuses, []evalspb.Status{evalspb.Status_PASSED, evalspb.Status_FAILED, evalspb.Status_PASSED}) {
		t.Fatalf("check statuses = %v, want pass/fail/pass", gotStatuses)
	}
	if !reflect.DeepEqual(gotMessages, []string{"", "second fails", ""}) {
		t.Fatalf("check messages = %v, want only broken rule message", gotMessages)
	}
	if cases[0].GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("case status = %v, want FAILED", cases[0].GetStatus())
	}
	if run.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("run status = %v, want FAILED", run.GetStatus())
	}
}

func TestIntegrationSuite_usesFreshValidatorPerCase(t *testing.T) {
	t.Parallel()

	run, err := NewIntegrationSuite("integration-isolation").
		AddCase("first", func(_ context.Context, v *validation.Validator) {
			v.Custom("first rule", true)
		}).
		AddCase("second", func(_ context.Context, v *validation.Validator) {
			v.Custom("second rule", true)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cases := run.GetIntegrationTest().GetCases()
	if len(cases) != 2 {
		t.Fatalf("case count = %d, want 2", len(cases))
	}
	for i, want := range []string{"first rule", "second rule"} {
		checks := cases[i].GetChecks()
		if len(checks) != 1 {
			t.Fatalf("case[%d] checks = %d, want 1: %v", i, len(checks), checks)
		}
		if checks[0].GetId() != want {
			t.Fatalf("case[%d] check id = %q, want %q", i, checks[0].GetId(), want)
		}
	}
}

func TestIntegrationSuite_zeroRuleCaseIsNotEvaluated(t *testing.T) {
	t.Parallel()

	run, err := NewIntegrationSuite("integration-empty").
		AddCase("empty", noopIntegrationCase).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cases := run.GetIntegrationTest().GetCases()
	if len(cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(cases))
	}
	if cases[0].GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("case status = %v, want NOT_EVALUATED", cases[0].GetStatus())
	}
	if run.GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("run status = %v, want NOT_EVALUATED", run.GetStatus())
	}
}

func TestIntegrationSuite_recoversCasePanic(t *testing.T) {
	t.Parallel()

	run, err := NewIntegrationSuite("integration-panic").
		AddCase("panics", func(context.Context, *validation.Validator) {
			panic("boom")
		}).
		AddCase("continues", func(_ context.Context, v *validation.Validator) {
			v.Custom("later case runs", true)
		}).
		Run(context.Background(), WithMaxConcurrency(1))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cases := run.GetIntegrationTest().GetCases()
	if len(cases) != 2 {
		t.Fatalf("case count = %d, want 2", len(cases))
	}
	if cases[0].GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("panic case status = %v, want FAILED", cases[0].GetStatus())
	}
	if len(cases[0].GetChecks()) != 1 {
		t.Fatalf("panic checks = %d, want 1", len(cases[0].GetChecks()))
	}
	panicCheck := cases[0].GetChecks()[0]
	if panicCheck.GetId() != "_evals.panic" {
		t.Fatalf("panic check id = %q, want _evals.panic", panicCheck.GetId())
	}
	if !strings.Contains(panicCheck.GetMessage(), "boom") {
		t.Fatalf("panic check message = %q, want panic value", panicCheck.GetMessage())
	}
	if cases[1].GetStatus() != evalspb.Status_PASSED {
		t.Fatalf("second case status = %v, want PASSED", cases[1].GetStatus())
	}
	if run.GetStatus() != evalspb.Status_FAILED {
		t.Fatalf("run status = %v, want FAILED", run.GetStatus())
	}
}

func TestIntegrationSuite_cancellationStopsNewCasesAndReturnsPartialRun(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var started atomic.Int32
	run, err := NewIntegrationSuite("integration-cancel").
		AddCase("first", func(_ context.Context, v *validation.Validator) {
			started.Add(1)
			v.Custom("first completed", true)
			cancel()
		}).
		AddCase("second", func(_ context.Context, v *validation.Validator) {
			started.Add(1)
			v.Custom("second should not start", true)
		}).
		Run(ctx, WithMaxConcurrency(1))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if run == nil {
		t.Fatal("Run() returned nil run, want partial run")
	}
	if started.Load() != 1 {
		t.Fatalf("started cases = %d, want 1", started.Load())
	}
	cases := run.GetIntegrationTest().GetCases()
	if len(cases) != 2 {
		t.Fatalf("case count = %d, want 2", len(cases))
	}
	if cases[0].GetStatus() != evalspb.Status_PASSED {
		t.Fatalf("first case status = %v, want PASSED", cases[0].GetStatus())
	}
	if cases[1].GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("second case status = %v, want NOT_EVALUATED", cases[1].GetStatus())
	}
	if run.GetStatus() != evalspb.Status_NOT_EVALUATED {
		t.Fatalf("run status = %v, want NOT_EVALUATED", run.GetStatus())
	}
}

func TestIntegrationSuite_recordsCaseAndRunTiming(t *testing.T) {
	base := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	ticks := make(chan time.Time, 6)
	for _, tick := range []time.Time{
		base,
		base.Add(10 * time.Millisecond),
		base.Add(35 * time.Millisecond),
		base.Add(50 * time.Millisecond),
		base.Add(70 * time.Millisecond),
		base.Add(80 * time.Millisecond),
	} {
		ticks <- tick
	}

	oldNow := now
	now = func() time.Time { return <-ticks }
	t.Cleanup(func() { now = oldNow })

	run, err := NewIntegrationSuite("integration-timing").
		AddCase("one", func(_ context.Context, v *validation.Validator) {
			v.Custom("one", true)
		}).
		AddCase("two", func(_ context.Context, v *validation.Validator) {
			v.Custom("two", true)
		}).
		Run(context.Background(), WithMaxConcurrency(1))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got, want := run.GetStartTime().AsTime(), base; !got.Equal(want) {
		t.Fatalf("run start = %v, want %v", got, want)
	}
	if got, want := run.GetEndTime().AsTime(), base.Add(80*time.Millisecond); !got.Equal(want) {
		t.Fatalf("run end = %v, want %v", got, want)
	}
	cases := run.GetIntegrationTest().GetCases()
	if got, want := cases[0].GetDuration().AsDuration(), 25*time.Millisecond; got != want {
		t.Fatalf("case[0] duration = %v, want %v", got, want)
	}
	if got, want := cases[1].GetDuration().AsDuration(), 20*time.Millisecond; got != want {
		t.Fatalf("case[1] duration = %v, want %v", got, want)
	}
}

func TestIntegrationSuite_panicMessageFormatsValues(t *testing.T) {
	t.Parallel()

	panicValue := struct {
		Code int
		Text string
	}{Code: 7, Text: "bad"}
	run, err := NewIntegrationSuite("integration-panic-format").
		AddCase("panics", func(context.Context, *validation.Validator) {
			panic(panicValue)
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	check := run.GetIntegrationTest().GetCases()[0].GetChecks()[0]
	want := fmt.Sprint(panicValue)
	if !strings.Contains(check.GetMessage(), want) {
		t.Fatalf("panic message = %q, want %q", check.GetMessage(), want)
	}
}
