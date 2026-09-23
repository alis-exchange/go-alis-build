---
title: Run wire contract
description: Result contract compatibility notes.
tags: [wire, proto, parity]
---

# Run wire contract

The API surface changed, while existing `alis.evals.v1.Run` fields keep their
branch-native placement and types. Parity tests compare normalized outputs for
all four branches against committed binary/JSON baselines. UUIDs, timestamps,
and the approved additive validation fields are normalized for comparison.

Existing result branches:

- `IntegrationTestResults`
- `AgentEvalResults`
- `LoadTestResults`
- `InfraObservationResults`

Integration cases use `checks`.

Specialized cases also expose additive `validations` fields:

- `AgentEvalResults.Case.validations`
- `LoadTestResults.Case.validations`
- `InfraObservationResults.Case.validations`

These additive fields carry developer/framework validation outcomes without
moving existing result fields.

`Check.id` and `Validation.id` are always the rule description. A failed
check or validation's `message` is the rule's failure detail when the rule
carries one (`validation.CustomRule.WithMessage`, read through
`interface{ Message() string }`), and the rule description otherwise. Passed
entries have an empty message. Rules without a detail produce the same output
as before, so integration check-message parity is unchanged for them.
