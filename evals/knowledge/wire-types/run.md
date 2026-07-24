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

One authoring limitation is explicit: `validation.Validator` provides an
integration rule description and satisfied state, but not a separate legacy
failed-check message. Integration check-message parity is therefore normalized;
failed check messages use the rule description.
