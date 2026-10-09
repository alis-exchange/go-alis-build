// Package adk runs evaluation sets exposed by an ADK (Agent Development Kit)
// sublauncher and converts its responses to protobuf-native eval results.
//
// The package does not register suites or publish results. The recommended
// path runs each ADK case as an evals.AgentEvalSuite case: [Provider.ListCases]
// lists the case ids of a set, [Provider.RunCase] runs one case, and
// [ProviderCase.RecordTo] records it into the suite case's builder through
// [CaseRecorder]. [Provider.Run] and [ProviderResult.Run] remain for callers
// that want one AgentEvalResults branch per eval set.
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
// # Running ADK cases in an AgentEvalSuite
//
//	ids, err := provider.ListCases(ctx, set)
//	if err != nil {
//	    return err
//	}
//	suite := evals.NewAgentEvalSuite(set)
//	for _, id := range ids {
//	    suite.AddCase(adk.SuiteCaseName(id), func(ctx context.Context, r *evals.AgentEvalResult) {
//	        res, err := provider.RunCase(ctx, set, id)
//	        if err != nil {
//	            r.Fail(err)
//	            return
//	        }
//	        res.RecordTo(r)
//	    })
//	}
//	run, err := suite.Run(ctx, evals.WithMaxConcurrency(4))
//
// The case closure captures the loop variable id. That is safe because Go 1.22
// and later give each loop iteration its own variable; this module declares
// go 1.26. Code built with an older go directive must copy id inside the loop.
//
// Each suite case is its own ADK run, so durations are measured per case and
// suite concurrency and cancellation apply. Case ids become
// "{suite}.{SuiteCaseName(id)}" and follow the order [Provider.ListCases]
// returns (the HTTP launcher sorts by id). [SuiteCaseName] replaces "." with
// "_" because suite case names cannot contain ".".
//
// ADK FAILED makes the suite case FAILED. ADK NOT_EVALUATED makes the suite
// case NOT_EVALUATED with its session, metrics and judge kept, through
// AgentEvalResult.SetNotEvaluated; it is never PASSED, and a failed metric or
// validation still makes it FAILED. The run is then NOT_EVALUATED unless
// another case failed. [ProviderCase.RecordTo] documents the rule.
//
// Each case declares its own judge. Mixing judge models or model versions
// across cases in one suite fails the conflicting cases with an _evals.judge
// validation.
//
// Listing needs a client that implements [CaseLister]. [HTTPClient] does; a
// custom [Client] without it makes [Provider.ListCases] return
// [ErrCaseListingUnsupported].
//
// # Run envelopes and publication
//
// [ProviderResult.Run] is deprecated. It supplies identity, timestamps, branch
// data, and status rollup. The caller owns metadata and publication:
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
