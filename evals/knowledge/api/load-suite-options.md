---
type: API Reference
title: Load-suite options
description: Options accepted by `NewLoadSuite`. Kept separate from `SuiteOption` because several test/eval options have no sensible load-test semantics.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/load.go
tags: [api, options, load]
timestamp: 2026-07-08T00:00:00Z
---

# `LoadSuiteOption`

| Option | Effect |
| ------ | ------ |
| `evals.WithLoadEnv(names ...string)` | Declare shared environments. Same semantics as `WithEnv` on test/eval suites. |
| `evals.WithLoadSetup(hook suite.SuiteHook)` | Suite-level pre-cases hook. Failure fails every case with a `setup` marker. |
| `evals.WithLoadTeardown(hook suite.SuiteHook)` | Suite-level post-cases hook. Errors logged, ignored. |
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

`StopOnFailure` and `WithContext` do not apply to load suites:

- **No `StopOnFailure`**: load cases run sequentially and a failed
  case does not invalidate the next case's measurement.
- **No per-suite `ContextDecorator`**: load suites always run under
  whatever context the runner-level decorator installs — the goal is
  to measure the SUT under the same context production traffic uses.
  Use runner-level context decoration for cross-suite defaults.

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
