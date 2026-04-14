package edge

import (
	"net/http"
	"testing"

	"go.alis.build/iam/v3"
)

func TestStripIdentityHeaders(t *testing.T) {
	headers := http.Header{
		iam.AlisAuthenticatedUserIDHeader:      {"svc"},
		iam.AlisAuthenticatedUserEmailHeader:   {"svc@example.com"},
		iam.AlisAuthenticatedIdentityCtxHeader: {"ctx"},
		iam.AlisUserIDHeader:                   {"12345"},
		iam.AlisUserEmailHeader:                {"user@example.com"},
		iam.AlisIdentityContextHeader:          {"ctx"},
		"X-Other":                              {"keep"},
	}

	StripIdentityHeaders(headers)

	if got := headers.Get(iam.AlisAuthenticatedUserIDHeader); got != "" {
		t.Fatalf("authenticated user id = %q, want empty", got)
	}
	if got := headers.Get(iam.AlisUserIDHeader); got != "" {
		t.Fatalf("caller user id = %q, want empty", got)
	}
	if got := headers.Get("X-Other"); got != "keep" {
		t.Fatalf("other header = %q, want keep", got)
	}
}

func TestApplyAuthenticatedIdentity(t *testing.T) {
	headers := http.Header{
		iam.AlisAuthenticatedUserIDHeader: {"stale"},
	}

	err := ApplyAuthenticatedIdentity(headers, iam.NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com"))
	if err != nil {
		t.Fatalf("ApplyAuthenticatedIdentity() error = %v", err)
	}

	if got := headers.Get(iam.AlisAuthenticatedUserIDHeader); got != "svc" {
		t.Fatalf("authenticated user id = %q, want svc", got)
	}
	if got := headers.Get(iam.AlisAuthenticatedUserEmailHeader); got != "alis-build@test-project.iam.gserviceaccount.com" {
		t.Fatalf("authenticated user email = %q, want deployment service account", got)
	}
}

func TestApplyCallerIdentityClearsHeadersWhenNil(t *testing.T) {
	headers := http.Header{
		iam.AlisUserIDHeader:          {"12345"},
		iam.AlisUserEmailHeader:       {"user@example.com"},
		iam.AlisIdentityContextHeader: {"ctx"},
	}

	err := ApplyCallerIdentity(headers, nil)
	if err != nil {
		t.Fatalf("ApplyCallerIdentity() error = %v", err)
	}

	if got := headers.Get(iam.AlisUserIDHeader); got != "" {
		t.Fatalf("caller user id = %q, want empty", got)
	}
	if got := headers.Get(iam.AlisUserEmailHeader); got != "" {
		t.Fatalf("caller user email = %q, want empty", got)
	}
}

func TestPrepareForwardedHeaders(t *testing.T) {
	headers := http.Header{
		iam.AlisAuthenticatedUserIDHeader: {"stale-auth"},
		iam.AlisUserIDHeader:              {"stale-caller"},
	}

	err := PrepareForwardedHeaders(
		headers,
		iam.NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com"),
		iam.NewIdentity("12345", "user@example.com"),
	)
	if err != nil {
		t.Fatalf("PrepareForwardedHeaders() error = %v", err)
	}

	if got := headers.Get(iam.AlisAuthenticatedUserIDHeader); got != "svc" {
		t.Fatalf("authenticated user id = %q, want svc", got)
	}
	if got := headers.Get(iam.AlisUserIDHeader); got != "12345" {
		t.Fatalf("caller user id = %q, want 12345", got)
	}
}

func TestPrepareForwardedHeadersRejectsNilHeaders(t *testing.T) {
	err := PrepareForwardedHeaders(nil, iam.NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com"), nil)
	if err == nil {
		t.Fatalf("PrepareForwardedHeaders() error = nil, want error")
	}
}
