---
title: Run wire contract
description: Result contract compatibility notes.
tags: [wire, proto, parity]
---

# Run wire contract

The API surface changed, but emitted `alis.evals.v1.Run` values remain
compatible with the frozen P0 fixtures. Parity tests compare all four branch
outputs against committed binary/JSON baselines.

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
