---
title: integration suites
description: Integration suite authoring with validation.Validator.
tags: [integration, validation]
---

# Integration suites

Integration cases use exactly:

```go
func(context.Context, *validation.Validator)
```

Example:

```go
suite := evals.NewIntegrationSuite("checkout-regression").
    AddCase("creates-order", func(ctx context.Context, v *validation.Validator) {
        order, err := client.CreateOrder(ctx, req)
        v.Custom("grpc.status_ok", err == nil)
        v.Custom("order.id_present", order.GetId() != "")
    })

run, err := suite.Run(ctx)
```

At case end the evals runtime reads `validator.Rules()` and writes integration
`checks`. Broken rules fail the case. A case with no rules is
`NOT_EVALUATED`.

Each case receives a fresh validator. Rule declaration order becomes check
order. A check's `id` is always the rule description, so it stays stable for
case history and trends. A failed check's `message` is the rule's failure
detail when one was set with `CustomRule.WithMessage`, and the rule
description otherwise. Passed checks have an empty message, even when a detail
was set.

```go
order, err := client.CreateOrder(ctx, req)
v.Custom("grpc.status_ok", err == nil).WithMessage(status.Convert(err).Message())
```

Normal Go errors remain normal inside the case. Add an explicit validation rule
when an error should affect the evaluation result.
