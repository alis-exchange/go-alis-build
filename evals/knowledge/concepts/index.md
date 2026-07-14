# Concepts

Reusable abstractions that appear throughout the framework.

* [Suite](/concepts/suite.md) - a named group of related cases plus optional environment dependencies and lifecycle hooks.
* [Case](/concepts/case.md) - the unit of execution — a `func(ctx, *T)` (test/eval) or a `ResultTarget` + SLOs (load).
* [T recorder](/concepts/t-recorder.md) - per-case handle that records assertion leaves for test and eval cases.
* [Environment](/concepts/environment.md) - shared setup/teardown identified by name and activated once per LRO.
* [Registry](/concepts/registry.md) - process-global publish point that RPCs consume, mirroring `http.DefaultServeMux`.
* [Reporter](/concepts/reporter.md) - sink that receives each completed `Run` proto.
* [Run](/concepts/run.md) - the top-level wire envelope covering all three suite kinds.
* [Status](/concepts/status.md) - the four-value enum (`STATUS_UNSPECIFIED`, `PASSED`, `FAILED`, `NOT_EVALUATED`) used everywhere.
