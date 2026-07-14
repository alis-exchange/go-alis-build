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
| `(*Suite).Case(name string, fn CaseFunc) error` | Register a test or eval case. Name must not contain `.` and must be unique inside the suite. Returns a typed error on failure. |
| `(*Suite).MustCase(name string, fn CaseFunc) *Suite` | Panicking variant that returns the receiver for fluent chaining. |
| `(*LoadSuite).LoadCase(name string, target ResultTarget, slos []SLO, opts ...LoadCaseOption) error` | Register a load case. Returns a typed error on failure. |
| `(*LoadSuite).MustLoadCase(name string, target ResultTarget, slos []SLO, opts ...LoadCaseOption) *LoadSuite` | Panicking variant that returns the receiver for fluent chaining. |

# Load case options

| Option | Effect |
| ------ | ------ |
| `evals.WithLoadCaseTags(map[string]string)` | Attach labels to the case wire result (`Case.tags`). |
| `evals.WithLoadCaseData(data ...any)` | Round-robin payloads rotated by request number. |
| `evals.WithLoadCaseDataProvider(p DataProvider)` | Programmatic per-request data. |

# Types

```go
type CaseFunc func(ctx context.Context, t *T)
type ResultTarget = loadgen.ResultTarget
type Profile  = loadgen.Profile
type SLO      struct { … opaque … }
```

# Validation

- Nil suite receiver → `suite.ErrNilSuite`.
- Nil case function → `evals.ErrNilCaseFunc`.
- Nil load target → `evals.ErrNilTarget`.
- Empty case name → `suite.ErrInvalidCaseName`.
- Name containing `.` → `suite.ErrInvalidCaseName`.
- Duplicate case name within the same suite → `suite.ErrDuplicateCase`.

The `Must*` variants wrap these into a panic when a registration failure
should halt the process.

# Chaining

The `Must*` methods return the suite receiver so chains like this compile:

```go
evals.MustNewIntegrationSuite("s").
    MustCase("a", handleA).
    MustCase("b", handleB)
```

For error-returning registration:

```go
s, err := evals.NewIntegrationSuite("s")
if err != nil { return err }
if err := s.Case("a", handleA); err != nil { return err }
if err := s.Case("b", handleB); err != nil { return err }
```

# Related

* [T recorder](/concepts/t-recorder.md)
* [T methods](/api/t-methods.md)
* [SLO constructors](/api/slo-constructors.md)

# Citations

[1] [suite.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/suite.go)
[2] [load.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/load.go)
