# `iam/v3`

`v3` is a transport-neutral IAM layer for gRPC, JSON-RPC, and HTTPS.

## Design Changes From `v2`

- Identity extraction is separated from transport wiring.
- Authorization no longer relies on an implicit "claimed token" skip.
- Downstream caller assertion is explicit.
- The active permission is explicit when creating an `Authorizer`.
- Policy fetching is transport-neutral through `PolicyFetcher`.

## Recommended Flow

1. Normalize transport headers into `ctx`.
2. Create an `Authorizer` with the explicit permission being checked.
3. Add identity or resource policies.
4. Call `Require()`.

Examples:

```go
ctx = iam.ContextWithGRPCMetadata(ctx)
authz, err := iamInstance.NewAuthorizer(ctx, "/library.v1.BooksService/GetBook")
if err != nil {
	return err
}
if err := authz.AddIdentityPolicy(); err != nil {
	return err
}
if err := authz.AddPolicyFromResource("publishers/123/books/456"); err != nil {
	return err
}
if err := authz.Require(); err != nil {
	return err
}
```

```go
ctx = iam.ContextWithHTTPRequest(ctx, req)
authz, err := iamInstance.NewAuthorizer(ctx, "books.get")
```

```go
ctx = iam.ContextWithJSONRPC(ctx, headers, "Books.Get")
authz, err := iamInstance.NewAuthorizer(ctx, "books.get")
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

`AsCaller` asserts caller identity using:

- `x-alis-user-id`
- `x-alis-user-email`
- `x-alis-identity-context` as base64 JSON for optional embedded policy

These headers should only be trusted when the authenticated caller is a configured trusted internal service or proxy.

`AsServiceAccount` does not forward caller identity headers.

## Gateway Pattern

For an edge gateway:

1. Authenticate the external bearer token.
2. Strip any incoming `x-alis-user-*` and `x-alis-identity-context` headers.
3. Build a trusted caller identity with `iam.NewIdentity(...)`.
4. Write asserted headers with `identity.AssertedHeaders()`.

For A2A service params, normalize them with `iam.ContextWithA2AServiceParams(...)` so downstream auth logic can use the same extraction path as HTTP, JSON-RPC, and gRPC.
