# `iam/v3`

`v3` is a transport-neutral IAM layer for gRPC, JSON-RPC, and HTTPS.

## Design Changes From `v2`

- Identity extraction is separated from transport wiring.
- Authorization no longer relies on an implicit "claimed token" skip.
- Downstream caller assertion is explicit.
- The active permission is explicit when creating an `Authorizer`.
- Policy fetching is transport-neutral through `PolicyFetcher`.

## Recommended Flow

1. Authenticate the inbound request in trusted gateway or auth middleware.
2. Forward the authenticated transport principal in trusted internal headers or attach it in context with `iam.WithAuthenticatedIdentity(...)`.
3. Normalize HTTP request metadata with `iam.HTTPMiddleware(...)` when handlers only receive `*http.Request`.
4. Create an `Authorizer` with the explicit permission being checked.
5. Add identity or resource policies.
6. Call `Require()`.

Examples:

```go
func (s *BooksServer) GetBook(ctx context.Context, req *librarypb.GetBookRequest) (*librarypb.GetBookResponse, error) {
	authz, err := iamInstance.NewAuthorizer(ctx, "/library.v1.BooksService/GetBook")
	if err != nil {
		return nil, err
	}
	if err := authz.AddIdentityPolicy(); err != nil {
		return nil, err
	}
	if err := authz.AddPolicyFromResource("publishers/123/books/456"); err != nil {
		return nil, err
	}
	if err := authz.Require(); err != nil {
		return nil, err
	}
	return &librarypb.GetBookResponse{}, nil
}
```

```go
handler := iam.HTTPMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	authz, err := iamInstance.NewAuthorizer(req.Context(), "books.get")
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	_ = authz
}))
```

```go
grpcServer := grpc.NewServer(
	grpc.UnaryInterceptor(iam.UnaryServerInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo) (*iam.Identity, error) {
		return iam.NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com"), nil
	})),
)
_ = grpcServer
```

`iam/v3` does not authenticate bearer tokens itself. It requires a previously
verified transport identity from trusted headers or context and uses normalized
transport headers for trusted internal caller assertion.

## Edge Helpers

Gateway and proxy-specific header preparation helpers live in
`go.alis.build/iam/v3/edge`.

```go
err := edge.PrepareForwardedHeaders(
	req.Header,
	iam.NewIdentity("svc", "alis-build@test-project.iam.gserviceaccount.com"),
	iam.NewIdentity("12345", "user@example.com"),
)
if err != nil {
	return err
}
```

## Explicit Outbound Identity

When making downstream calls, choose the intent explicitly:

```go
outboundCtx := iam.AsCaller(ctx)
headers, err := iam.OutgoingHTTPHeaders(outboundCtx)
```

```go
outboundCtx := iam.AsServiceAccount(ctx)
headers, err := iam.OutgoingHTTPHeaders(outboundCtx)
```

```go
outboundCtx, err := iam.AsCallerGRPC(ctx)
if err != nil {
	return err
}

resp, err := booksClient.GetBook(outboundCtx, &librarypb.GetBookRequest{
	Name: "publishers/123/books/456",
})
if err != nil {
	return err
}
_ = resp
```

`AsCaller` asserts caller identity using:

- `x-alis-user-id`
- `x-alis-user-email`
- `x-alis-identity-context` as base64 JSON for optional embedded policy

Authenticated transport identity can be forwarded separately using:

- `x-alis-authenticated-user-id`
- `x-alis-authenticated-user-email`
- `x-alis-authenticated-identity-context` as base64 JSON for optional embedded policy

These headers should only be trusted when the authenticated caller is a configured trusted internal service or proxy.

`AsServiceAccount` does not forward caller identity headers.

## Gateway Pattern

For an edge gateway:

1. Authenticate the external bearer token.
2. Strip any incoming `x-alis-authenticated-*`, `x-alis-user-*`, and `x-alis-identity-context` headers.
3. Build the authenticated transport identity with `iam.NewIdentity(...)`.
4. Write authenticated headers with `identity.AuthenticatedHeaders()`.
5. If proxying on behalf of a user, build the caller identity with `iam.NewIdentity(...)`.
6. Write asserted caller headers with `identity.AssertedHeaders()`.

For A2A service params, normalize them with `iam.ContextWithA2AServiceParams(...)` so downstream auth logic can use the same extraction path as HTTP, JSON-RPC, and gRPC.
