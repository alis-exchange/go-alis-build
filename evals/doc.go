// Package evals is the case-authoring surface for integration tests, agent
// evaluations, and load tests. Suites written against this package are
// picked up by the deployed TestServiceServer and executed via three
// LRO-backed RPCs (RunIntegrationTest, RunAgentEval, RunLoadTest).
//
// The framework is split into a small authoring package (this one) and a
// set of runtime subpackages that consumers rarely import directly:
// [suite], [registry], [runner], [mapper], [report], [execution],
// [loadgen], and the case-adjacent helpers in [env], [auth], and [adk].
// See the top-level README for a wiring diagram and the end-to-end
// lifecycle.
//
// # Suites and cases
//
// Everything is organised around Suites — named groups of Cases that share
// optional environment setup, lifecycle hooks, and caller identity. Three
// kinds of suite exist, one per run type on the TestService RPC:
//
//   - Integration test suite ([NewSuite] / [RegisterIntegration]) — cases
//     assert against a live deployment; results surface as `Checks`.
//   - Agent-eval suite ([NewEvalSuite] / [RegisterEval]) — cases exercise
//     an agent transcript and grade with rubrics/scores; results surface as
//     `Metrics`.
//   - Load-test suite ([NewLoadSuite] / [RegisterLoad]) — the framework
//     generates traffic against a target function and evaluates SLOs on the
//     aggregate metrics; results surface as per-case `Summary` + `Checks`.
//
// Every case is qualified as `{suite}.{case}` at registration and can be
// selected from the RPC via case_ids ("suite" for the whole suite,
// "suite.case" for one case).
//
// # Integration tests
//
// An integration case is a func(ctx, *T) that measures the SUT with [Call]
// and records assertions on [T]. Every recording method on T returns
// whether the leaf passed, so authors guard-and-return without any custom
// flow control:
//
//	func Register() {
//	    s := evals.NewSuite("example-v1",
//	        evals.WithEnv("example-v1"),
//	        evals.WithSetup(seedExample),
//	        evals.WithTeardown(cleanupExample),
//	    )
//	    s.Case("get-item", func(ctx context.Context, t *evals.T) {
//	        r := evals.Call(ctx, func(ctx context.Context) (*examplepb.Item, error) {
//	            return clients.Example.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
//	        })
//	        if !t.NoErr("grpc", r.Err) {
//	            return
//	        }
//	        t.Max("latency", r.Latency, 500*time.Millisecond)
//	        t.Check("has-name", r.Resp.GetName() != "")
//	    })
//	    evals.RegisterIntegration(s)
//	}
//
// # Agent evaluations
//
// Eval cases receive the same [T] but their recorded leaves are surfaced
// as `Metric`s on the wire, so [T.Score] is the primary tool. Authors
// bring their own scoring: a canned unigram scorer is available via
// [Rouge1F1] and LLM-judges are plain Go calls that feed a score into
// t.Score.
//
//	s := evals.NewEvalSuite("example-agent-v1",
//	    evals.WithEnv("agent-runtime"),
//	    evals.WithIdentity(iam.SystemIdentity),
//	)
//	s.Case("golden-summary", func(ctx context.Context, t *evals.T) {
//	    r := evals.Call(ctx, func(ctx context.Context) (*agentpb.Reply, error) {
//	        return clients.Agent.Chat(ctx, prompt)
//	    })
//	    if !t.NoErr("transport", r.Err) {
//	        return
//	    }
//	    t.Score("rouge-1", evals.Rouge1F1(r.Resp.GetText(), golden), 0.5, "vs golden")
//	})
//	evals.RegisterEval(s)
//
// Agent evaluations can also be sourced lazily via [RegisterAgent] +
// [registry.AgentEvalProvider]; the [adk] subpackage provides a provider
// that discovers eval sets from a deployed ADK agent.
//
// # Load tests
//
// Load suites are shaped differently: cases are not func(ctx, *T) but a
// single [Target] the framework invokes many times under a resolved
// [Profile]. There is no [T]; the assertions are SLOs declared alongside
// the target. See [SLOLatencyP50], [SLOLatencyP95], [SLOLatencyP99],
// [SLOErrorRate], and [SLOMinQPS].
//
//	s := evals.NewLoadSuite("example-v1-load", evals.WithLoadEnv("example-v1"))
//	s.LoadCase("list-items",
//	    func(ctx context.Context) error {
//	        _, err := clients.Example.ListItems(ctx, &examplepb.ListItemsRequest{PageSize: 5})
//	        return err
//	    },
//	    evals.SLOLatencyP99(500*time.Millisecond),
//	    evals.SLOErrorRate(0.01),
//	)
//	evals.RegisterLoad(s)
//
// The `mode` field on RunLoadTest (MINIMAL … LUDICROUS) picks a preset
// profile via [DefaultLoadProfile]. Suites can override presets with
// [WithLoadProfile] when a specific case needs bespoke concurrency,
// duration, or warmup. A resolved profile fully replaces the default (no
// field-level merging); this keeps intent explicit at high modes.
//
// # Suite options
//
// Test and eval suites share a [SuiteOption] set:
//
//   - [WithEnv](names...) — declare shared environments the suite requires.
//     Environments must have been registered via [env.Register] before the
//     suite is constructed.
//   - [WithSetup](hook) / [WithTeardown](hook) — run before/after the
//     suite's cases. Setup failure fails every case with a setup-error
//     marker and skips teardown.
//   - [WithIdentity](identity) — simulate a specific caller for every RPC
//     issued by the suite's cases. Uses [auth.Outgoing] to attach identity
//     headers on outgoing gRPC calls.
//   - [StopOnFailure]() — mark the suite so a failing case causes the
//     remaining cases to be recorded NOT_EVALUATED. Use for stateful
//     flows where later cases have no meaning after a preceding step
//     fails.
//
// Load suites use a separate [LoadSuiteOption] set to keep semantics
// clear: [WithLoadEnv], [WithLoadSetup], [WithLoadTeardown],
// [WithLoadProfile]. There is no `StopOnFailure` for load — load cases
// within a suite run sequentially anyway (concurrent load windows would
// contaminate each other's measurements), and one case failing does not
// invalidate the next case's measurement.
//
// # Environments
//
// Environments are shared setup steps identified by name. Register them
// with [env.Register] once (typically in package init) and reference by
// name in every suite that needs the state:
//
//	env.Register("example-v1",
//	    env.WithSetup(seedExample),
//	    env.WithTeardown(cleanupExample),
//	)
//
// Environments are activated once per RunTest/RunEval/RunLoad LRO, not
// per suite. This lets several suites share expensive setup (seeding a
// database, warming a cache) without paying the cost repeatedly.
//
// # Authentication
//
// Identity is propagated to the SUT via three headers set by the [auth]
// subpackage:
//
//   - `x-alis-identity`     — marshaled iam.Identity
//   - `x-alis-forwarded-authorization` — the identity's unsigned JWT
//   - Cloud Run invoker auth is added separately by go.alis.build/client/v2
//
// [WithIdentity] on a suite is the usual entry point. If no identity is
// set on the suite or the runner, RPCs go out with the caller's identity
// as-received.
//
// # Assertion primitives
//
// [T] exposes a small vocabulary that covers what production suites
// typically need:
//
//   - `Check(id, pass)` / `Checkf(id, pass, format, ...)` — boolean.
//   - `NoErr(id, err)` — records `err.Error()` on failure.
//   - `Max(id, got, limit)` — records the observed vs limit duration.
//   - `Score(id, score, threshold, rationale)` — for eval cases.
//
// Every method returns the pass boolean so authors can chain guards:
//
//	if !t.NoErr("grpc", r.Err) { return }
//	if !t.Max("latency", r.Latency, budget) { return }
//	t.Check("shape", r.Resp.GetName() != "")
//
// Duplicate check IDs within one case are surfaced as a single
// [DuplicateCheckIDName] leaf so results stay parseable downstream.
//
// # Registration and execution
//
// Suites are published to a process-wide registry via [RegisterIntegration],
// [RegisterEval], [RegisterLoad], and [RegisterAgent] (lazy providers).
// The default registry is accessible via [DefaultRegistry] and is what
// TestServiceServer consumes when it starts a run — the wiring matches
// http.DefaultServeMux.
//
// Each RunTest/RunEval/RunLoad RPC starts an LRO, selects matching
// suites from the registry, hands them to [runner.Runner], maps completed
// suites to `evalspb.Run` via [mapper], and emits every Run to
// [report.Reporter].
//
// # ADK launcher requirement
//
// The evals framework serves its HTTP surface (agent eval discovery,
// run_eval, and the LRO callbacks) under the `/api/` prefix. That mux is
// installed by the sublauncher at go.alis.build/adk/launchers/evals:
// binaries embedding this framework must call the launcher during
// startup or the RPCs will 404. Add the launcher import once alongside
// your other neuron wiring:
//
//	import _ "go.alis.build/adk/launchers/evals"
//
// See the README for the end-to-end flow, including how consumers wire
// custom reporters and override load profiles.
package evals
