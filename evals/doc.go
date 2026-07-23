// Package evals runs explicitly named integration, agent-evaluation, load,
// and infrastructure-observation suites.
//
// Suites are defined directly in Go with NewIntegrationSuite,
// NewAgentEvalSuite, NewLoadSuite, or NewInfraObservationSuite. AddCase returns
// the suite for chained definition. Run executes synchronously without
// publishing; RunAndPublish executes and reports the materialized protobuf run.
//
// Suite definitions seal on first execution and may then be run repeatedly or
// concurrently. Cases execute sequentially by default. WithMaxConcurrency
// permits bounded parallel execution while preserving result order.
//
// Developers own setup, cleanup, clients, credentials, fixtures, and
// higher-level control flow using ordinary Go. The package does not provide
// registries or framework-managed lifecycle hooks.
package evals
