---
type: Reference
title: Filter grammar
description: How `case_ids` selects suites and cases at the RPC boundary.
tags: [filter, case_ids, selection]
timestamp: 2026-07-08T00:00:00Z
---

# Grammar

Every RPC (`RunIntegrationTest`, `RunAgentEval`, `RunLoadTest`,
`RunInfraObservation`) accepts
a `case_ids` list. Each entry is either a suite name or a
`suite.case` pair:

| Filter entry | Selects |
| ------------ | ------- |
| _empty list_ | Every registered suite of the requested kind. |
| `"my-suite"` | Every case in `my-suite`. Its `WithSetup` / `WithTeardown` run exactly once. |
| `"my-suite.foo"` | Just the `foo` case in `my-suite`. |

# Multiple entries

Multiple entries **union**:

```
["example-v1.get-item", "example-v1.list-empty-parent"]
```

selects two cases. Mixing suite-scoped and case-scoped filters
against the same suite promotes to whole-suite selection.

# Validation

Unknown suite/case ids are rejected synchronously at the RPC boundary
with `InvalidArgument`. `evalspb.ValidateSelection` (also exposed as
`Registry.ValidateSelection`) is what performs this check — **no LRO
is created for invalid inputs**.

# Malformed entries

- Empty string → `suite.ErrInvalidFilterPath`.
- More than one `.` → `suite.ErrInvalidFilterPath`.
- Entry referencing a suite of the wrong kind → suite-not-found for
  the requested RPC.

# Related

* [Registry](/concepts/registry.md)
* [Suite](/concepts/suite.md)

# Citations

[1] [README — Filter grammar](https://github.com/alis-exchange/go-alis-build/blob/main/evals/README.md#filter-grammar)
[2] [registry/registry.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/registry/registry.go)
