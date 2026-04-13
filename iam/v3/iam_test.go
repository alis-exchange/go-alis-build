package iam

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

func TestGRPCAssertedCallerBecomesEffectiveIdentity(t *testing.T) {
	iam := newTestIAM(t)
	serviceToken := newTestJWT(t, map[string]any{
		"sub":   "svc",
		"email": "alis-build@test-project.iam.gserviceaccount.com",
	})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(AuthHeader, "Bearer "+serviceToken))
	ctx = ContextWithGRPCMetadata(ctx)
	ctx = WithRequestIdentity(context.Background(), mustNewRequestIdentity(t, iam, ctx, "12345", "user@example.com", nil))
	outgoingMD, err := OutgoingGRPCMetadata(AsCaller(ctx))
	if err != nil {
		t.Fatalf("OutgoingGRPCMetadata() error = %v", err)
	}

	ctx = metadata.NewIncomingContext(context.Background(), outgoingMD.Copy())
	ctx = metadata.NewIncomingContext(ctx, metadata.Join(metadata.Pairs(AuthHeader, "Bearer "+serviceToken), outgoingMD))
	ctx = ContextWithGRPCMetadata(ctx)

	authz, err := iam.NewAuthorizer(ctx, "books.get")
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}

	if got := authz.Caller().Email(); got != "user@example.com" {
		t.Fatalf("Caller().Email() = %q, want %q", got, "user@example.com")
	}
	if got := authz.Authenticated().Email(); got != "alis-build@test-project.iam.gserviceaccount.com" {
		t.Fatalf("Authenticated().Email() = %q, want deployment service account", got)
	}
}

func TestRequireUsesExplicitAssertedCaller(t *testing.T) {
	iam := newTestIAM(t)
	userPolicy := &iampb.Policy{
		Bindings: []*iampb.Binding{{
			Role:    "roles/books.viewer",
			Members: []string{"user:12345"},
		}},
	}
	deploymentServiceToken := newTestJWT(t, map[string]any{
		"sub":   "svc",
		"email": "alis-build@test-project.iam.gserviceaccount.com",
	})
	regularServiceToken := newTestJWT(t, map[string]any{
		"sub":   "svc-2",
		"email": "worker@test-project.iam.gserviceaccount.com",
	})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		AuthHeader, "Bearer "+deploymentServiceToken,
		AlisUserIDHeader, "12345",
		AlisUserEmailHeader, "user@example.com",
		AlisIdentityContextHeader, mustAssertedIdentityContextHeader(t, userPolicy),
	))
	ctx = ContextWithGRPCMetadata(ctx)

	authz, err := iam.NewAuthorizer(ctx, "books.get")
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	if err := authz.AddIdentityPolicy(); err != nil {
		t.Fatalf("AddIdentityPolicy() error = %v", err)
	}
	if err := authz.Require(); err != nil {
		t.Fatalf("Require() error = %v", err)
	}

	serviceOnlyCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		AuthHeader, "Bearer "+regularServiceToken,
	))
	serviceOnlyCtx = ContextWithGRPCMetadata(serviceOnlyCtx)

	serviceOnlyAuthz, err := iam.NewAuthorizer(serviceOnlyCtx, "books.get")
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	if err := serviceOnlyAuthz.Require(userPolicy); err == nil {
		t.Fatalf("Require() error = nil, want permission denied")
	} else {
		var deniedErr *PermissionDeniedError
		if !errors.As(err, &deniedErr) {
			t.Fatalf("Require() error = %T, want *PermissionDeniedError", err)
		}
		if deniedErr.Permission != "books.get" {
			t.Fatalf("PermissionDeniedError.Permission = %q, want %q", deniedErr.Permission, "books.get")
		}
		if deniedErr.Caller != "serviceAccount:worker@test-project.iam.gserviceaccount.com" {
			t.Fatalf("PermissionDeniedError.Caller = %q, want worker service account", deniedErr.Caller)
		}
	}
}

func TestOutgoingAuthMustBeExplicit(t *testing.T) {
	iam := newTestIAM(t)
	serviceToken := newTestJWT(t, map[string]any{
		"sub":   "svc",
		"email": "alis-build@test-project.iam.gserviceaccount.com",
	})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		AuthHeader, "Bearer "+serviceToken,
		AlisUserIDHeader, "12345",
		AlisUserEmailHeader, "user@example.com",
	))
	authz, err := iam.NewAuthorizer(ContextWithGRPCMetadata(ctx), "books.get")
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	baseCtx := WithRequestIdentity(context.Background(), authz.RequestIdentity())

	if _, err := OutgoingHTTPHeaders(baseCtx); err == nil {
		t.Fatalf("OutgoingHTTPHeaders() error = nil, want explicit outbound auth error")
	}

	headers, err := OutgoingHTTPHeaders(AsCaller(baseCtx))
	if err != nil {
		t.Fatalf("OutgoingHTTPHeaders(AsCaller) error = %v", err)
	}
	if got := headers.Get(AlisUserIDHeader); got != "12345" {
		t.Fatalf("x-alis-user-id = %q, want %q", got, "12345")
	}
	if got := headers.Get(AlisUserEmailHeader); got != "user@example.com" {
		t.Fatalf("x-alis-user-email = %q, want %q", got, "user@example.com")
	}
	if got := headers.Get(AlisIdentityContextHeader); got != "" {
		t.Fatalf("x-alis-identity-context = %q, want empty when no policy is present", got)
	}

	headers, err = OutgoingJSONRPCHeaders(AsServiceAccount(baseCtx))
	if err != nil {
		t.Fatalf("OutgoingJSONRPCHeaders(AsServiceAccount) error = %v", err)
	}
	if got := headers.Get(AlisUserIDHeader); got != "" {
		t.Fatalf("service account call asserted caller header = %q, want empty", got)
	}
}

func TestAssertedHeadersAndStrip(t *testing.T) {
	userPolicy := &iampb.Policy{
		Bindings: []*iampb.Binding{{
			Role:    "roles/books.viewer",
			Members: []string{"user:12345"},
		}},
	}
	identity := NewIdentity("12345", "user@example.com", WithIdentityPolicy(userPolicy))

	headers, err := identity.AssertedHeaders()
	if err != nil {
		t.Fatalf("AssertedHeaders() error = %v", err)
	}
	if got := headers.Get(AlisUserIDHeader); got != "12345" {
		t.Fatalf("x-alis-user-id = %q, want %q", got, "12345")
	}
	if got := headers.Get(AlisUserEmailHeader); got != "user@example.com" {
		t.Fatalf("x-alis-user-email = %q, want %q", got, "user@example.com")
	}
	if got := headers.Get(AlisIdentityContextHeader); got == "" {
		t.Fatalf("x-alis-identity-context = empty, want populated context")
	}

	StripAssertedIdentityHeaders(headers)
	if got := headers.Get(AlisUserIDHeader); got != "" {
		t.Fatalf("x-alis-user-id after strip = %q, want empty", got)
	}
	if got := headers.Get(AlisUserEmailHeader); got != "" {
		t.Fatalf("x-alis-user-email after strip = %q, want empty", got)
	}
	if got := headers.Get(AlisIdentityContextHeader); got != "" {
		t.Fatalf("x-alis-identity-context after strip = %q, want empty", got)
	}
}

func TestA2AServiceParamsMetadataExtraction(t *testing.T) {
	iam := newTestIAM(t)
	userToken := newTestJWT(t, map[string]any{
		"sub":   "svc",
		"email": "alis-build@test-project.iam.gserviceaccount.com",
	})

	serviceParams := map[string][]string{
		AuthHeader:          {"Bearer " + userToken},
		AlisUserIDHeader:    {"12345"},
		AlisUserEmailHeader: {"user@example.com"},
	}
	ctx := ContextWithA2AServiceParams(context.Background(), serviceParams, "message/send")
	identity, err := iam.ExtractRequestIdentity(ctx)
	if err != nil {
		t.Fatalf("ExtractRequestIdentity(a2a) error = %v", err)
	}
	if got := identity.Caller.Email(); got != "user@example.com" {
		t.Fatalf("Caller().Email() = %q, want %q", got, "user@example.com")
	}
}

func TestHTTPAndJSONRPCMetadataExtraction(t *testing.T) {
	iam := newTestIAM(t)
	userToken := newTestJWT(t, map[string]any{
		"sub":   "12345",
		"email": "user@example.com",
	})

	httpReq, err := http.NewRequest(http.MethodGet, "https://example.com/books/1", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	httpReq.Header.Set(AuthHeader, "Bearer "+userToken)
	httpCtx := ContextWithHTTPRequest(context.Background(), httpReq)
	httpIdentity, err := iam.ExtractRequestIdentity(httpCtx)
	if err != nil {
		t.Fatalf("ExtractRequestIdentity(http) error = %v", err)
	}

	jsonHeaders := http.Header{}
	jsonHeaders.Set(AuthHeader, "Bearer "+userToken)
	jsonCtx := ContextWithJSONRPC(context.Background(), jsonHeaders, "Books.Get")
	jsonIdentity, err := iam.ExtractRequestIdentity(jsonCtx)
	if err != nil {
		t.Fatalf("ExtractRequestIdentity(jsonrpc) error = %v", err)
	}

	if httpIdentity.Caller.Email() != jsonIdentity.Caller.Email() {
		t.Fatalf("caller mismatch: http=%q jsonrpc=%q", httpIdentity.Caller.Email(), jsonIdentity.Caller.Email())
	}
}

func newTestIAM(t *testing.T) *IAM {
	t.Helper()
	iam, err := New([]*Role{{
		Name:        "roles/books.viewer",
		Permissions: []string{"books.get"},
	}}, WithProjectID("test-project"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return iam
}

func newTestJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	headerBytes, err := json.Marshal(map[string]any{
		"alg": "none",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("json.Marshal(header) error = %v", err)
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(headerBytes) + "." + base64.RawURLEncoding.EncodeToString(payloadBytes) + ".sig"
}

func newTestJWTWithPolicy(t *testing.T, claims map[string]any, policy *iampb.Policy) string {
	t.Helper()
	policyBytes, err := proto.Marshal(policy)
	if err != nil {
		t.Fatalf("proto.Marshal(policy) error = %v", err)
	}
	claims["policy"] = base64.StdEncoding.EncodeToString(policyBytes)
	return newTestJWT(t, claims)
}

func mustAssertedIdentityContextHeader(t *testing.T, policy *iampb.Policy) string {
	t.Helper()

	payload := &assertedIdentityContext{}
	if policy != nil {
		policyBytes, err := proto.Marshal(policy)
		if err != nil {
			t.Fatalf("proto.Marshal(policy) error = %v", err)
		}
		payload.Policy = base64.StdEncoding.EncodeToString(policyBytes)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(payloadBytes)
}

func mustNewRequestIdentity(t *testing.T, iam *IAM, authenticatedCtx context.Context, callerID string, callerEmail string, policy *iampb.Policy) *RequestIdentity {
	t.Helper()

	authenticated, err := iam.ExtractRequestIdentity(authenticatedCtx)
	if err != nil {
		t.Fatalf("ExtractRequestIdentity(authenticatedCtx) error = %v", err)
	}

	caller := NewIdentity(callerID, callerEmail, WithIdentityPolicy(policy))

	return &RequestIdentity{
		Caller:        caller,
		Authenticated: authenticated.Authenticated,
	}
}
