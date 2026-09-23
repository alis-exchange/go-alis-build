package evals

import (
	"context"
	"reflect"
	"strings"
	"testing"

	evalspb "go.alis.build/common/alis/evals"
	"go.alis.build/validation"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestIntegrationSuite_failedCheckUsesRuleMessage(t *testing.T) {
	t.Parallel()

	run, err := NewIntegrationSuite("integration-messages").
		AddCase("messages", func(_ context.Context, v *validation.Validator) {
			v.Custom("get.ok", false).WithMessage("NotFound: skill with the given id not found")
			v.Custom("list.ok", false)
			v.Custom("create.ok", true).WithMessage("unused detail")
			v.Custom("delete.ok", false).WithMessage("")
		}).
		Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cases := run.GetIntegrationTest().GetCases()
	if len(cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(cases))
	}
	gotIDs := []string{}
	gotMessages := []string{}
	for _, check := range cases[0].GetChecks() {
		gotIDs = append(gotIDs, check.GetId())
		gotMessages = append(gotMessages, check.GetMessage())
	}
	if !reflect.DeepEqual(gotIDs, []string{"get.ok", "list.ok", "create.ok", "delete.ok"}) {
		t.Fatalf("check ids = %v, want rule descriptions", gotIDs)
	}
	wantMessages := []string{
		"NotFound: skill with the given id not found",
		"list.ok",
		"",
		"delete.ok",
	}
	if !reflect.DeepEqual(gotMessages, wantMessages) {
		t.Fatalf("check messages = %v, want %v", gotMessages, wantMessages)
	}
}

func TestValidationsFromValidator_failedValidationUsesRuleMessage(t *testing.T) {
	t.Parallel()

	v := validation.NewValidator()
	v.Custom("p95.under_budget", false).WithMessage("p95 was 812ms, budget 500ms")
	v.Custom("errors.none", false)
	v.Custom("rps.reached", true).WithMessage("unused detail")

	got := validationsFromValidator(v)
	gotIDs := []string{}
	gotMessages := []string{}
	for _, val := range got {
		gotIDs = append(gotIDs, val.GetId())
		gotMessages = append(gotMessages, val.GetMessage())
	}
	if !reflect.DeepEqual(gotIDs, []string{"p95.under_budget", "errors.none", "rps.reached"}) {
		t.Fatalf("validation ids = %v, want rule descriptions", gotIDs)
	}
	if !reflect.DeepEqual(gotMessages, []string{"p95 was 812ms, budget 500ms", "errors.none", ""}) {
		t.Fatalf("validation messages = %v, want detail, description fallback, empty", gotMessages)
	}
}

func TestRunAndPublish_publishedRunCarriesCheckMessage(t *testing.T) {
	rec := &publicationRecorder{}

	_, err := NewIntegrationSuite("publish-message").
		AddCase("case", func(_ context.Context, v *validation.Validator) {
			v.Custom("get.ok", false).WithMessage("NotFound: skill with the given id not found")
		}).
		RunAndPublish(context.Background(), WithReporter(rec))
	if err != nil {
		t.Fatalf("RunAndPublish() error = %v", err)
	}
	if rec.reports != 1 {
		t.Fatalf("reports = %d, want 1", rec.reports)
	}

	data, err := protojson.Marshal(rec.last)
	if err != nil {
		t.Fatalf("protojson.Marshal() error = %v", err)
	}
	var decoded evalspb.Run
	if err := protojson.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("protojson.Unmarshal() error = %v", err)
	}
	checks := decoded.GetIntegrationTest().GetCases()[0].GetChecks()
	if len(checks) != 1 {
		t.Fatalf("check count = %d, want 1", len(checks))
	}
	if got := checks[0].GetId(); got != "get.ok" {
		t.Fatalf("published check id = %q, want get.ok", got)
	}
	if got := checks[0].GetMessage(); got != "NotFound: skill with the given id not found" {
		t.Fatalf("published check message = %q, want the detail", got)
	}
	if !strings.Contains(string(data), "NotFound: skill with the given id not found") {
		t.Fatalf("published JSON does not carry the detail: %s", data)
	}
}
