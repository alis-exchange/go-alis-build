// Package adk integrates the ADK (Agent Development Kit) evaluation
// sublauncher into the evals runtime as a lazy [registry.AgentEvalProvider].
//
// Instead of registering static eval suites in Go, an ADK-backed agent
// publishes its eval sets as data. This package discovers those sets over
// HTTP at run time, filters them against the incoming case_ids, invokes
// them via the sublauncher, and adapts the responses into the internal
// [execution] and wire types the runner and mapper already understand.
//
// # Wiring
//
// Register the ADK provider once at process init. Configuration lives on
// the [Agent] struct that [NewProvider] wraps — there is no separate
// registration table. Match the [Agent] fields to the deployed agent's
// URL and app name (published by the ADK launcher):
//
//	provider := adk.NewProvider(adk.Agent{
//	    BaseURL:    "https://example-agent-...run.app",
//	    PathPrefix: "/api", // default; override if the launcher was mounted elsewhere
//	    AppName:    "example.agent.v1",
//	    DefaultMetrics: []models.EvalMetric{
//	        adk.ResponseMatchScore(0.7),
//	    },
//	})
//	if err := evals.RegisterAgent(provider); err != nil {
//	    panic(err)
//	}
//
// Case_ids follow the standard filter grammar. `agent-set` selects an
// entire eval set; `agent-set.case-id` selects one entry inside it.
//
// # Dependencies
//
// This package depends on
// `go.alis.build/adk/launchers/evals/evaluation/models` for the metric
// types only, not the launcher's runtime code. Neuron binaries that
// serve these evals must additionally import the launcher itself once
// (typically as a blank import) so that the sublauncher's `/api/`
// handlers — including `list_eval_sets` and `run_eval` — are installed
// on the default mux:
//
//	import _ "go.alis.build/adk/launchers/evals"
//
// All HTTP calls from this package are plain net/http; JSON payloads
// mirror the sublauncher's public API.
//
// # Context and authentication
//
// This package is transport-agnostic. [NewHTTPClient] accepts any
// [http.RoundTripper] via [WithTransport]; callers install whatever auth
// their ADK sublauncher requires — bearer tokens, oauth2, Cloud Run ID
// tokens, mTLS, or nothing at all for unauthenticated local endpoints:
//
//	client := adk.NewHTTPClient(baseURL,
//	    adk.WithTransport(myAuthTransport),
//	    adk.WithTimeout(30*time.Minute),
//	)
//
// [Provider] uses an unauthenticated client by default. Wire a custom
// factory via [WithClientFactory] when the sublauncher requires auth.
//
// [AudienceFromBaseURL] is a URL helper for consumers minting Cloud Run
// ID tokens against a Cloud Run-hosted sublauncher; the client itself
// does not use it.
package adk
