---
type: API Reference
title: Registration functions
description: Publish suites and providers to the process-wide registry that `TestServiceServer` consumes.
resource: https://github.com/alis-exchange/go-alis-build/blob/main/evals/register.go
tags: [api, register, registry]
timestamp: 2026-07-08T00:00:00Z
---

# Functions

| Function | Effect |
| -------- | ------ |
| `evals.RegisterIntegration(s *Suite) error` | Publish an integration suite. Returns `suite.ErrNilSuite` if `s == nil` or `evals.ErrWrongSuiteKind` if `s.Kind() != KindTest`. |
| `evals.RegisterEval(s *Suite) error` | Publish an eval suite. Returns `suite.ErrNilSuite` if `s == nil` or `evals.ErrWrongSuiteKind` if `s.Kind() != KindEval`. |
| `evals.RegisterLoad(s *LoadSuite) error` | Publish a load suite. Returns `suite.ErrNilSuite` if `s == nil`. |
| `evals.RegisterInfraObserve(s *InfraObserveSuite) error` | Publish an infra observation suite. Returns `suite.ErrNilSuite` if `s == nil`. |
| `evals.RegisterAgent(p registry.AgentEvalProvider) error` | Publish a lazy agent-eval provider (for example an ADK-backed one). Returns `evals.ErrNilProvider` if `p == nil`. |
| `evals.DefaultRegistry() *registry.Registry` | Return the process-wide registry that `TestServiceServer` consumes. Useful for tests. |

# When to call

Register at `init()` time. All registration functions target
`evals.DefaultRegistry()`. Callers must handle the returned error (or
`log.Fatal`); the framework does not panic on registration errors.

# The lazy-provider path

`RegisterAgent` publishes a `registry.AgentEvalProvider` — a lazy
provider that produces cases at run time rather than at registration
time. The canonical implementation is `adk.NewProvider`, which
discovers eval sets over HTTP against a deployed ADK agent. See
[ADK agent eval](/suites/adk-agent-eval.md).

# Registry access for tests

`DefaultRegistry()` returns the same registry all `RegisterXxx`
functions target. Tests can:

- Assert what a package registered at init:
  ```go
  reg := evals.DefaultRegistry()
  // Query reg for suites, cases, providers.
  ```
- Construct a private registry for isolation (the `registry` package
  is exported).

# Related

* [Registry concept](/concepts/registry.md)
* [`registry` package](/packages/registry.md)
* [Suite constructors](/api/suite-constructors.md)

# Citations

[1] [register.go](https://github.com/alis-exchange/go-alis-build/blob/main/evals/register.go)
