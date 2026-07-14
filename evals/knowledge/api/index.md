# API reference

Every exported symbol on the framework's public authoring surface,
grouped by purpose.

# Constructing and configuring

* [Suite constructors](/api/suite-constructors.md) - `NewSuite`, `NewEvalSuite`, `NewLoadSuite`.
* [Shared suite options](/api/suite-options.md) - options accepted by both `NewSuite` and `NewEvalSuite`.
* [Load-suite options](/api/load-suite-options.md) - options accepted by `NewLoadSuite`.
* [Case registration](/api/case-registration.md) - `Case`, `LoadCase`, and their types.

# Authoring cases

* [T methods](/api/t-methods.md) - the full method vocabulary on `*T`.
* [SLO constructors](/api/slo-constructors.md) - latency, error-rate, and throughput SLOs.
* [Load profile](/api/load-profile.md) - `Profile` struct fields and semantics.
* [Helpers](/api/helpers.md) - `Call`, streaming helpers, `Result[T]`, `Rouge1F1`, load-profile resolution.

# Registration and wiring

* [Environment API](/api/environment.md) - `env.Register`, `env.WithSetup`, `env.WithTeardown`, `env.Get`.
* [Registration functions](/api/registration.md) - `RegisterIntegration`, `RegisterEval`, `RegisterLoad`, `RegisterAgent`, `DefaultRegistry`.
* [Reporters](/api/reporters.md) - `Reporter` interface and bundled implementations.

# Errors

* [Errors](/api/errors.md) - `EvalError` interface, helpers, and concrete error types.
