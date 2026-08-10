// Package adk runs evaluation sets exposed by an ADK (Agent Development Kit)
// sublauncher and converts its responses to protobuf-native eval results.
//
// The package does not register suites or publish results. [Provider.Run]
// discovers matching eval sets at call time and returns one [ProviderResult]
// per set. Each result contains the suite name, observed start and end times,
// and an AgentEvalResults protobuf branch. [ProviderResult.Run] builds the
// complete Run envelope without publishing it.
//
// # Running a provider
//
// Configuration lives on the [Agent] passed to [NewProvider]:
//
//	provider := adk.NewProvider(adk.Agent{
//	    BaseURL:    "https://example-agent-...run.app",
//	    PathPrefix: "/api",
//	    AppName:    "example.agent.v1",
//	    DefaultMetrics: []models.EvalMetric{
//	        adk.ResponseMatchScore(0.7),
//	    },
//	    JudgeModel:        "gemini-2.5-pro",
//	    JudgeModelVersion: "2025-06-05",
//	})
//	state, err := adk.SessionStateFromProto(req.GetSessionState())
//	results, err := provider.Run(ctx, filters, adk.WithSessionState(state))
//
// The provider returns normal Go errors. Callers decide whether an error should
// stop their workflow or be represented as evaluation data.
//
// Case filters use the standard ADK grammar: "agent-set" selects an entire
// eval set and "agent-set.case-id" selects one case within it.
//
// # Run-level session state
//
// [WithSessionState] forwards initial ADK session state on every run_eval
// call. [SessionStateFromProto] converts alis.evals.v1 RunAgentEvalRequest
// session_state to the map shape the ADK sublauncher expects.
//
// # Run envelopes and publication
//
// [ProviderResult.Run] supplies identity, timestamps, branch data, and status
// rollup. The caller owns metadata and publication:
//
//	run := result.Run()
//	run.Operation = operation
//	err := reporter.ReportRun(ctx, run)
//
// Envelope construction does not give the ADK adapter suite lifecycle or
// reporter ownership. [ProviderResult.Results] remains available when callers
// need only the protobuf branch.
//
// ADK exposes only total eval-set elapsed time. [Provider] divides that time
// evenly across returned cases, so case durations are an approximation; the
// provider result's start and end times preserve the measured set duration.
//
// # Judge provenance
//
// [Agent.JudgeModel] is authoritative when set. Otherwise the provider probes
// metric criteria in declaration order and uses the first configured judge
// model. Set [Agent.JudgeModel] explicitly when stable wire provenance matters.
//
// Judge call count is synthesized from LLM-as-judge metric result entries, not
// backend invocations, and should be treated as a lower-bound observation.
//
// # Dependencies
//
// This package imports ADK evaluation models but not the launcher runtime.
// Neuron binaries serving the sublauncher must install its handlers, typically
// with:
//
//	import _ "go.alis.build/adk/launchers/evals"
//
// # Context and authentication
//
// [NewHTTPClient] accepts a custom [http.RoundTripper] through [WithTransport].
// [Provider] uses an unauthenticated client by default; use [WithClientFactory]
// to create an authenticated client when required.
//
// [AudienceFromBaseURL] helps callers mint Cloud Run ID tokens. The ADK client
// itself remains transport-agnostic.
package adk
