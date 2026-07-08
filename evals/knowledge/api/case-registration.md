---
type: API Reference
title: Case registration
description: Methods for adding cases to a suite. Returns the suite receiver so calls can be chained.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go
tags: [api, case]
timestamp: 2026-07-08T00:00:00Z
---

# Methods

| Method | Effect |
| ------ | ------ |
| `(*Suite).Case(name string, fn CaseFunc) *Suite` | Register a test or eval case. Name must not contain `.` and must be unique inside the suite. Returns the receiver for chaining. |
| `(*LoadSuite).LoadCase(name string, target Target, slos ...SLO) *LoadSuite` | Register a load case. `target` is `func(ctx context.Context) error`. |

# Types

```go
type CaseFunc func(ctx context.Context, t *T)
type Target   = loadgen.Target    // func(ctx context.Context) error
type Profile  = loadgen.Profile
type SLO      struct { … opaque … }
```

# Validation

- Empty case name → panic (`suite.ErrInvalidCaseName`).
- Name containing `.` → panic (`suite.ErrInvalidCaseName`).
- Duplicate case name within the same suite → panic
  (`suite.ErrDuplicateCase`).

# Chaining

Both methods return the suite receiver so chains like this compile:

```go
evals.NewSuite("s").
    Case("a", handleA).
    Case("b", handleB)
```

# Related

* [T recorder](/concepts/t-recorder.md)
* [T methods](/api/t-methods.md)
* [SLO constructors](/api/slo-constructors.md)

# Citations

[1] [suite.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go)
[2] [load.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/load.go)
