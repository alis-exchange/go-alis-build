---
type: Go Package
title: package evals/errors
description: The EvalError interface plus ToGRPC/ToGRPCf/IsEval/Code helpers.
resource: https://github.com/alis-exchange/go-alis-build/tree/main/evals/errors
tags: [package, errors, grpc]
timestamp: 2026-07-08T00:00:00Z
---

# Role

`evals/errors` defines the `EvalError` interface and helpers for
translating framework errors into gRPC statuses at the RPC boundary.

# Public surface

```go
type EvalError interface {
    error
    GRPCStatus() *status.Status
}

func ToGRPC(err error) error
func ToGRPCf(field string, err error) error
func IsEval(err error) bool
func Code(err error) codes.Code
```

# Files

| File | Purpose |
| ---- | ------- |
| `errors.go` | Interface and helper functions. |
| `doc.go` | Package documentation. |
| `errors_test.go` | Tests. |

# Related

* [Errors API](/api/errors.md)

# Citations

[1] [evals/errors tree](https://github.com/alis-exchange/go-alis-build/tree/main/evals/errors)
