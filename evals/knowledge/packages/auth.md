---
type: Go Package
title: package evals/auth
description: Outgoing gRPC identity headers for eval-issued RPCs.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/auth
tags: [package, auth, identity]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/auth` centralises the machinery for attaching caller identity
to every RPC issued by an eval case. It is what `evals.WithIdentity`
routes through.

# Public surface

- `auth.Outgoing(ctx context.Context, id *iam.Identity) context.Context`
  — attaches `x-alis-identity` and `x-alis-forwarded-authorization`
  headers to the outgoing gRPC metadata.
- `auth.SystemOutgoing(ctx context.Context) context.Context` — the
  default: attaches `iam.SystemIdentity`.

# Layers

`evals/auth` does not touch Cloud Run invoker auth. That layer is
added by `go.alis.build/client/v2` on every outbound call
independently.

# Files

| File | Purpose |
| ---- | ------- |
| `auth.go` | `Outgoing`, `SystemOutgoing`, header names. |
| `doc.go` | Package documentation. |
| `auth_test.go` | Tests. |

# Related

* [Authentication (operations)](/operations/authentication.md)
* [WithIdentity option](/api/suite-options.md)

# Citations

[1] [evals/auth tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/auth)
