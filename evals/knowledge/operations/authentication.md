---
type: Reference
title: Authentication
description: Two independent layers — Cloud Run invoker auth and application-level caller identity.
tags: [auth, identity, iam]
timestamp: 2026-07-08T00:00:00Z
---

# Layers

Two independent layers of authentication apply to every RPC issued
by an eval case:

- **Cloud Run invoker auth** is added by `go.alis.build/client/v2` on
  every outbound gRPC call (OIDC ID token in the `Authorization`
  header). Nothing in this framework touches it.
- **Application-level caller identity** is what `evals.WithIdentity`
  controls. It is set on the outgoing context by `evals/auth.Outgoing`
  as two headers:
  - `x-alis-identity` — marshaled `iam.Identity`
  - `x-alis-forwarded-authorization` — `identity.UnsignedJWT`

If no `WithIdentity` is set on the suite, the runner uses
`iam.SystemIdentity`. Load suites **always** run with system identity
— the goal is to measure the SUT under the same identity production
traffic uses.

# Simulating a caller

```go
s := evals.MustNewIntegrationSuite("example-v1",
    evals.WithIdentity(myTestIdentity),
)
```

Every RPC issued from cases in this suite carries `myTestIdentity` in
the identity headers. Different suites in the same neuron can
simulate different callers.

# ADK agent evals

For ADK-backed agent evals the equivalent HTTP headers are set on
every request:

- `X-Serverless-Authorization` — Cloud Run ID token.
- `X-Alis-Identity` — marshaled `iam.Identity`.
- `X-Alis-Forwarded-Authorization` — `identity.UnsignedJWT`.

# Related

* [`auth` package](/packages/auth.md)
* [ADK agent eval](/suites/adk-agent-eval.md)
* [Shared suite options — WithIdentity](/api/suite-options.md)

# Citations

[1] [README — Authentication](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#authentication)
[2] [auth/auth.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/auth/auth.go)
