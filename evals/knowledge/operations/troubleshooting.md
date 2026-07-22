---
type: Operations
title: Troubleshooting
description: Map surfaced signals and typed errors to causes and fixes.
tags: [operations, diagnostics, errors]
timestamp: 2026-07-22T00:00:00Z
---

# Troubleshooting

When a run looks wrong on the dashboard or in logs, use this table before
reading source. Framework diagnostic ids live in
[`evals/verdict`](https://github.com/alis-exchange/go-alis-build/tree/main/evals/verdict).

## Framework diagnostics (`_evals.*`)

| Surfaced signal | Meaning | Fix |
| --- | --- | --- |
| `_evals.no-checks-recorded` | Case body ran but recorded no assertions | Add a `t.Check` / `t.NoErr`, or call `t.Pass(id)` when empty is intentional |
| `_evals.transport_errors` | Transport failures occurred with no error-rate SLO | Declare `SLOErrorRate`, or fix the SUT / network |
| `_evals.aborted` | Abort-on-SLO cancelled the load window early | Named SLO breached; compare observed vs limit on the failing check |
| `_evals.teardown` | Suite teardown failed | Inspect teardown logs; infra-observe carries this as a synthetic Cloud Run snapshot |
| `_evals.duplicate-check-id` | Two checks in one case reused the same id | Rename one check id |
| `_evals.reserved-check-id` | User check id uses the reserved `_evals.` prefix | Pick a different id |
| `_evals.setup` | Environment or suite setup failed before cases ran | Fix setup hook; selected cases are emitted as `FAILED` |
| `_evals.case` | Case panicked or returned an internal runner error | Read stack trace in service logs |
| `_evals.skipped` | Selected case skipped by `StopOnFailure` or cancellation | Expected after an upstream failure or cancelled LRO; filtered-out cases are absent |

## Infra observation `FetchStatus`

| Surfaced signal | Meaning | Fix |
| --- | --- | --- |
| `INFRA_FETCH_STATUS_PERMISSION_DENIED` | Monitoring API denied the query | Grant `roles/monitoring.viewer` (or broader) on the target project |
| `INFRA_FETCH_STATUS_TIMEOUT` | Monitoring query timed out | Retry; widen lookback only if the window is valid |
| `INFRA_FETCH_STATUS_UNAVAILABLE` | Transient Monitoring / client error | Retry; check target project and metric filters |
| `INFRA_FETCH_STATUS_OK` | Snapshot succeeded | — |

Standalone infra-observe cases **fail** when any target snapshot is not OK.
Load-integrated infra snapshots stay diagnostic-only in v1.

Suite teardown has no generic infra check collection, so an infra-observe case
encodes `_evals.teardown` as a synthetic `CloudRunTargetSnapshot`: `id` is
`_evals.teardown`, `fetch_status` is `INFRA_FETCH_STATUS_UNAVAILABLE`, and
`fetch_message` contains the teardown error. Environment teardown failures are
logged because they occur after all suite results have been materialized.

## Registration and selection errors

| Surfaced signal | Meaning | Fix |
| --- | --- | --- |
| `ErrRegistryFrozen` | Registration after [`registry.Freeze`](../../registry/registry.go) | Register suites and environments only during `init()` / before serving |
| `ErrUnknownCase` | `case_ids` filter matched nothing | Check `{suite}.{case}` spelling against registered ids |
| `ErrDuplicateSuite` | Same suite name registered twice | Use one registration path per suite name |
| `ErrUnknownEnvironments` | Suite references an env name not registered | Register env before suite; call `Freeze()` only after all envs exist |
| `ErrDuplicateCase` | Duplicate `{suite}.{case}` id | Rename the case |
| `ErrInvalidCaseName` | Case name empty or contains `.` | Use a short segment; suite prefix is added automatically |
| `ErrDuplicateSLOID` | Two SLOs on one load case share an id | Give each SLO a unique id |
| `ErrDualLoadCaseData` | Load case declares both `TransportTarget` and streaming target | Pick one invocation style per case |
| `ErrInvalidProfile` / `ErrInvalidSLO` | Load profile or SLO limits are NaN, Inf, or inconsistent | Fix numeric fields at registration or profile override time |

## Reporter and mapper friction

| Surfaced signal | Meaning | Fix |
| --- | --- | --- |
| Reporter error logged, LRO still completes | Sink failed under [`report.All`](../../report/report.go) or fail-fast skipped a later sink | Check sink IAM (`ALIS_OS_PRODUCT_PROJECT`), topic/dataset names, and timeouts |
| LRO stalls ~10s+ per suite | Serial [`report.All`](../../report/report.go) with multiple timed sinks | Expected worst case (sum of per-sink timeouts); use [`report.FailFast`](../../report/report.go) or fewer sinks |
| Empty `google_project_id` on Run | [`mapper.SetConfig`](../../mapper/config.go) not called | Set `mapper.Config{GoogleProjectID: os.Getenv("ALIS_OS_PROJECT")}` at bootstrap |
| Pub/Sub OK, BigQuery empty | Fan-out stopped at first error | Switch to `report.All` or fix the failing sink first |

## Related

* [End-to-end lifecycle](/operations/lifecycle.md)
* [Reporters](/api/reporters.md)
* [Registry](/concepts/registry.md)
