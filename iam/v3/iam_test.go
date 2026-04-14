package iam

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestGRPCAssertedCallerBecomesEffectiveIdentity(t *testing.T) {
	iam := newTestIAM(t)
	authenticated := NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com")
	ctx := WithRequestIdentity(context.Background(), &RequestIdentity{
		Caller:        NewIdentity("12345", "user@example.com"),
		Authenticated: authenticated,
	})
	outgoingMD, err := OutgoingGRPCMetadata(AsCaller(ctx))
	if err != nil {
		t.Fatalf("OutgoingGRPCMetadata() error = %v", err)
	}

	authenticatedHeaders, err := authenticated.AuthenticatedHeaders()
	if err != nil {
		t.Fatalf("AuthenticatedHeaders() error = %v", err)
	}
	ctx = metadata.NewIncomingContext(context.Background(), metadata.Join(metadataFromHTTPHeaders(authenticatedHeaders), outgoingMD))
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
	deploymentService := NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com")
	regularService := NewIdentity("svc-2", "worker@test-project.iam.gserviceaccount.com")
	deploymentHeaders, err := deploymentService.AuthenticatedHeaders()
	if err != nil {
		t.Fatalf("AuthenticatedHeaders() error = %v", err)
	}
	regularHeaders, err := regularService.AuthenticatedHeaders()
	if err != nil {
		t.Fatalf("AuthenticatedHeaders() error = %v", err)
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Join(metadataFromHTTPHeaders(deploymentHeaders), metadata.Pairs(
		AlisUserIDHeader, "12345",
		AlisUserEmailHeader, "user@example.com",
		AlisIdentityContextHeader, mustAssertedIdentityContextHeader(t, userPolicy),
	)))
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

	serviceOnlyCtx := metadata.NewIncomingContext(context.Background(), metadataFromHTTPHeaders(regularHeaders))

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
	authenticated := NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com")
	authenticatedHeaders, err := authenticated.AuthenticatedHeaders()
	if err != nil {
		t.Fatalf("AuthenticatedHeaders() error = %v", err)
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Join(metadataFromHTTPHeaders(authenticatedHeaders), metadata.Pairs(
		AlisUserIDHeader, "12345",
		AlisUserEmailHeader, "user@example.com",
	)))
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

func TestAuthenticatedHeadersAndStrip(t *testing.T) {
	identity := NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com")

	headers, err := identity.AuthenticatedHeaders()
	if err != nil {
		t.Fatalf("AuthenticatedHeaders() error = %v", err)
	}
	if got := headers.Get(AlisAuthenticatedUserIDHeader); got != "svc" {
		t.Fatalf("x-alis-authenticated-user-id = %q, want %q", got, "svc")
	}
	if got := headers.Get(AlisAuthenticatedUserEmailHeader); got != "alis-build@test-project.iam.gserviceaccount.com" {
		t.Fatalf("x-alis-authenticated-user-email = %q, want deployment service account", got)
	}

	StripAuthenticatedIdentityHeaders(headers)
	if got := headers.Get(AlisAuthenticatedUserIDHeader); got != "" {
		t.Fatalf("x-alis-authenticated-user-id after strip = %q, want empty", got)
	}
	if got := headers.Get(AlisAuthenticatedUserEmailHeader); got != "" {
		t.Fatalf("x-alis-authenticated-user-email after strip = %q, want empty", got)
	}
	if got := headers.Get(AlisAuthenticatedIdentityCtxHeader); got != "" {
		t.Fatalf("x-alis-authenticated-identity-context after strip = %q, want empty", got)
	}
}

func TestA2AServiceParamsMetadataExtraction(t *testing.T) {
	iam := newTestIAM(t)
	authenticated := NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com")
	authenticatedHeaders, err := authenticated.AuthenticatedHeaders()
	if err != nil {
		t.Fatalf("AuthenticatedHeaders() error = %v", err)
	}

	serviceParams := map[string][]string{
		AlisAuthenticatedUserIDHeader:    {authenticatedHeaders.Get(AlisAuthenticatedUserIDHeader)},
		AlisAuthenticatedUserEmailHeader: {authenticatedHeaders.Get(AlisAuthenticatedUserEmailHeader)},
		AlisUserIDHeader:                 {"12345"},
		AlisUserEmailHeader:              {"user@example.com"},
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
	authenticated := NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com")
	authenticatedHeaders, err := authenticated.AuthenticatedHeaders()
	if err != nil {
		t.Fatalf("AuthenticatedHeaders() error = %v", err)
	}

	httpReq, err := http.NewRequest(http.MethodGet, "https://example.com/books/1", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	httpReq.Header.Set(AlisAuthenticatedUserIDHeader, authenticatedHeaders.Get(AlisAuthenticatedUserIDHeader))
	httpReq.Header.Set(AlisAuthenticatedUserEmailHeader, authenticatedHeaders.Get(AlisAuthenticatedUserEmailHeader))
	httpReq.Header.Set(AlisUserIDHeader, "12345")
	httpReq.Header.Set(AlisUserEmailHeader, "user@example.com")
	httpCtx := ContextWithHTTPRequest(context.Background(), httpReq)
	httpIdentity, err := iam.ExtractRequestIdentity(httpCtx)
	if err != nil {
		t.Fatalf("ExtractRequestIdentity(http) error = %v", err)
	}

	jsonHeaders := http.Header{}
	jsonHeaders.Set(AlisAuthenticatedUserIDHeader, authenticatedHeaders.Get(AlisAuthenticatedUserIDHeader))
	jsonHeaders.Set(AlisAuthenticatedUserEmailHeader, authenticatedHeaders.Get(AlisAuthenticatedUserEmailHeader))
	jsonHeaders.Set(AlisUserIDHeader, "12345")
	jsonHeaders.Set(AlisUserEmailHeader, "user@example.com")
	jsonCtx := ContextWithJSONRPC(context.Background(), jsonHeaders, "Books.Get")
	jsonIdentity, err := iam.ExtractRequestIdentity(jsonCtx)
	if err != nil {
		t.Fatalf("ExtractRequestIdentity(jsonrpc) error = %v", err)
	}

	if httpIdentity.Caller.Email() != jsonIdentity.Caller.Email() {
		t.Fatalf("caller mismatch: http=%q jsonrpc=%q", httpIdentity.Caller.Email(), jsonIdentity.Caller.Email())
	}
}

func TestExtractRequestIdentityRequiresAuthenticatedIdentity(t *testing.T) {
	iam := newTestIAM(t)
	ctx := ContextWithHTTPRequest(context.Background(), httptestNewRequest(t))

	_, err := iam.ExtractRequestIdentity(ctx)
	if err == nil {
		t.Fatalf("ExtractRequestIdentity() error = nil, want authenticated identity error")
	}
	if got := err.Error(); got != "authenticated identity not found in context or trusted headers" {
		t.Fatalf("ExtractRequestIdentity() error = %q, want authenticated identity error", got)
	}
}

func TestUnaryServerInterceptorAttachesAuthenticatedIdentity(t *testing.T) {
	iam := newTestIAM(t)
	interceptor := UnaryServerInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo) (*Identity, error) {
		return NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com"), nil
	})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		AlisAuthenticatedUserIDHeader, "svc",
		AlisAuthenticatedUserEmailHeader, "alis-build@test-project.iam.gserviceaccount.com",
		AlisUserIDHeader, "12345",
		AlisUserEmailHeader, "user@example.com",
	))
	info := &grpc.UnaryServerInfo{FullMethod: "/library.v1.BooksService/GetBook"}

	resp, err := interceptor(ctx, "request", info, func(ctx context.Context, req any) (any, error) {
		authz, err := iam.NewAuthorizer(ctx, "books.get")
		if err != nil {
			return nil, err
		}
		if got := authz.Authenticated().Email(); got != "alis-build@test-project.iam.gserviceaccount.com" {
			t.Fatalf("Authenticated().Email() = %q, want deployment service account", got)
		}
		if got := authz.Caller().Email(); got != "user@example.com" {
			t.Fatalf("Caller().Email() = %q, want %q", got, "user@example.com")
		}
		if md, ok := RequestMetadataFromContext(ctx); !ok {
			t.Fatalf("RequestMetadataFromContext() ok = false, want true")
		} else if md.Transport != TransportGRPC {
			t.Fatalf("RequestMetadata.Transport = %q, want %q", md.Transport, TransportGRPC)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("interceptor() error = %v", err)
	}
	if resp != "ok" {
		t.Fatalf("interceptor() response = %v, want ok", resp)
	}
}

func TestUnaryServerInterceptorRejectsMissingIdentity(t *testing.T) {
	interceptor := UnaryServerInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo) (*Identity, error) {
		return nil, nil
	})

	_, err := interceptor(context.Background(), "request", &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		t.Fatalf("handler should not be called")
		return nil, nil
	})
	if err == nil {
		t.Fatalf("interceptor() error = nil, want unauthenticated")
	}
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("status.Code(err) = %s, want %s", got, codes.Unauthenticated)
	}
}

func TestHTTPMiddlewareAttachesAuthenticatedIdentity(t *testing.T) {
	iam := newTestIAM(t)
	middleware := HTTPMiddleware(func(req *http.Request) (*Identity, error) {
		return NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com"), nil
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		authz, err := iam.NewAuthorizer(req.Context(), "books.get")
		if err != nil {
			t.Fatalf("NewAuthorizer() error = %v", err)
		}
		if got := authz.Authenticated().Email(); got != "alis-build@test-project.iam.gserviceaccount.com" {
			t.Fatalf("Authenticated().Email() = %q, want deployment service account", got)
		}
		if got := authz.Caller().Email(); got != "user@example.com" {
			t.Fatalf("Caller().Email() = %q, want %q", got, "user@example.com")
		}
		if md, ok := RequestMetadataFromContext(req.Context()); !ok {
			t.Fatalf("RequestMetadataFromContext() ok = false, want true")
		} else if md.Transport != TransportHTTPS {
			t.Fatalf("RequestMetadata.Transport = %q, want %q", md.Transport, TransportHTTPS)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "https://example.com/books/1", nil)
	req.Header.Set(AlisAuthenticatedUserIDHeader, "svc")
	req.Header.Set(AlisAuthenticatedUserEmailHeader, "alis-build@test-project.iam.gserviceaccount.com")
	req.Header.Set(AlisUserIDHeader, "12345")
	req.Header.Set(AlisUserEmailHeader, "user@example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("ServeHTTP() status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestHTTPMiddlewareAllowsHeaderDerivedIdentityWhenResolverIsNil(t *testing.T) {
	iam := newTestIAM(t)
	middleware := HTTPMiddleware(nil)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		authz, err := iam.NewAuthorizer(req.Context(), "books.get")
		if err != nil {
			t.Fatalf("NewAuthorizer() error = %v", err)
		}
		if got := authz.Authenticated().Email(); got != "alis-build@test-project.iam.gserviceaccount.com" {
			t.Fatalf("Authenticated().Email() = %q, want deployment service account", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "https://example.com/books/1", nil)
	req.Header.Set(AlisAuthenticatedUserIDHeader, "svc")
	req.Header.Set(AlisAuthenticatedUserEmailHeader, "alis-build@test-project.iam.gserviceaccount.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("ServeHTTP() status = %d, want %d", rec.Code, http.StatusNoContent)
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

func httptestNewRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://example.com/books/1", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	return req
}

func metadataFromHTTPHeaders(headers http.Header) metadata.MD {
	md := metadata.MD{}
	for key, values := range headers {
		md[key] = append([]string(nil), values...)
	}
	return md
}
