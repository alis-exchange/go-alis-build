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
