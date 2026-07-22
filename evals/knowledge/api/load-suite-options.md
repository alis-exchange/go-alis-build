---
type: API Reference
title: Load-suite options
description: Options accepted by `NewLoadSuite`, including lifecycle, context, failure propagation, profiles, and infrastructure targets.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/load.go
tags: [api, options, load]
timestamp: 2026-07-08T00:00:00Z
---

# `LoadSuiteOption`

| Option | Effect |
| ------ | ------ |
| `evals.WithLoadEnv(names ...string)` | Declare shared environments. Same semantics as `WithEnv` on test/eval suites. |
| `evals.WithLoadSetup(hook suite.SuiteHook)` | Suite-level pre-cases hook. Failure fails every case with an `_evals.setup` marker. |
| `evals.WithLoadTeardown(hook suite.SuiteHook)` | Suite-level post-cases hook. Failure marks every completed case failed with an `_evals.teardown` diagnostic. |
| `evals.WithLoadContext(fn evals.ContextDecorator)` | Apply request-scoped identity, authentication, or tracing values to setup, teardown, and case traffic. |
| `evals.WithLoadStopOnFailure()` | Stop after the first non-passing load case and emit `_evals.skipped` for remaining cases. |
| `evals.WithLoadProfile(mode evalspb.RunLoadTestRequest_Mode, p Profile)` | Override the framework default profile for that specific mode. The override fully replaces the default; other modes keep theirs. Panics if `mode == MODE_UNSPECIFIED`. |
| `evals.WithCloudRunTargets(...)` | Declare Cloud Run infra targets. After each case, server-side snapshots are fetched over the measurement window and attached to `LoadTestResults.Case.cloud_run`. |
| `evals.WithSpannerTargets(...)` | Declare Spanner infra targets. Snapshots attach to `LoadTestResults.Case.spanner`. Role is always `DEPENDENCY` on the wire. |

# Infra targets on load suites

`WithCloudRunTargets` and `WithSpannerTargets` also apply to
[infra observe suites](/suites/infra-observe-suite.md). On load suites
they are optional diagnostics: case `status` is unchanged in v1;
`infra_checks` is empty on the wire.

Target `ID` values must be unique across Cloud Run and Spanner targets
in a suite. Each `WithCloudRunTargets` call must include exactly one
`RoleEntry` target.

# Why load has its own option set

Load suites use a distinct option type so load profiles and infrastructure
targets cannot be attached to integration or eval suites. Lifecycle, context
decoration, and StopOnFailure have load-specific constructors but retain the
same runner semantics as the other suite kinds.

# Override semantics

`WithLoadProfile(mode, profile)` **fully replaces** the framework
default for the given mode. Other modes retain their defaults. There
is no field-level merging — a partial override at high intensity is
easy to get wrong; the framework refuses to guess.

To override multiple modes, call `WithLoadProfile` multiple times.

# Related

* [Load profile fields](/api/load-profile.md)
* [Load mode presets](/operations/load-mode-presets.md)
* [Load suite](/suites/load-suite.md)

# Citations

[1] [load.go — LoadSuiteOption](https://github.com/alis-exchange/go-alis-build/blob/main/evals/load.go)
