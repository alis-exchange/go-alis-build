// Package evals is the case-authoring surface for integration tests, agent
// evaluations, and load tests. Suites written against this package are
// picked up by the deployed TestServiceServer and executed via three
// LRO-backed RPCs (RunIntegrationTest, RunAgentEval, RunLoadTest).
//
// The framework is split into a small authoring package (this one) and a
// set of runtime subpackages that consumers rarely import directly:
// [suite], [registry], [runner], [mapper], [report], [execution],
// [loadgen], and the case-adjacent helpers in [env] and [adk]. See the
// top-level README for a wiring diagram and the end-to-end lifecycle.
//
// # Suites and cases
//
// Everything is organised around Suites — named groups of Cases that share
// optional environment setup, lifecycle hooks, and caller identity. Three
// kinds of suite exist, one per run type on the TestService RPC:
//
//   - Integration test suite ([NewIntegrationSuite] / [RegisterIntegration]) —
//     cases assert against a live deployment; results surface as `Checks`.
//   - Agent-eval suite ([NewAgentEvalSuite] / [RegisterEval]) — cases exercise
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
// An integration case is a func(ctx, *T) that measures the SUT with [Call],
// [CallServerStream], or [CallClientStream] and records assertions on [T].
// Every recording method on T returns whether the leaf passed, so authors
// guard-and-return without any custom flow control:
//
//	func Register() {
//	    s := evals.MustNewIntegrationSuite("example-v1",
//	        evals.WithEnv("example-v1"),
//	        evals.WithSetup(seedExample),
//	        evals.WithTeardown(cleanupExample),
//	    )
//	    s.MustCase("get-item", func(ctx context.Context, t *evals.T) {
//	        r := evals.Call(ctx, func(ctx context.Context) (*examplepb.Item, error) {
//	            return clients.Example.GetItem(ctx, &examplepb.GetItemRequest{Name: rootItem})
//	        })
//	        if !t.NoErr("grpc", r.Err) {
//	            return
//	        }
//	        t.Max("latency", r.Latency, 500*time.Millisecond)
//	        t.Check("has-name", r.Resp.GetName() != "")
//	    })
//	    if err := evals.RegisterIntegration(s); err != nil {
//	        panic(err)
//	    }
//	}
//
// For server-streaming RPCs, use [CallServerStream] to drain a bounded stream
// and capture TTFB, total duration, and inter-message gaps. TTFB is 0 when
// no message is received — guard before asserting:
//
//	res := evals.CallServerStream(ctx, func(ctx context.Context) (grpc.ServerStreamingClient[Event], error) {
//	    return clients.Foo.Watch(ctx, req)
//	})
//	if !t.NoErr("grpc", res.Err) { return }
//	if len(res.Messages) > 0 {
//	    t.Max("ttfb", res.TTFB, 100*time.Millisecond)
//	}
//	t.Max("total", res.TotalDuration, 2*time.Second)
//	// MessageIntervals[i] is the gap between Messages[i] and Messages[i+1].
//
// Do not use [CallServerStream] on watch RPCs that never send EOF; use a
// context deadline to bound execution. If Recv returns a nil message with
// nil error, Err is set and partial messages are preserved.
//
// For client-streaming RPCs, use [CallClientStream] when send-side vs
// response-side timing matters. [Call] remains appropriate for unary-shaped
// client streams when split timing is not needed.
//
//	r := evals.CallClientStream(ctx,
//	    func(ctx context.Context) (grpc.ClientStreamingClient[Chunk, UploadResult], error) {
//	        return clients.Example.Upload(ctx)
//	    },
//	    func(stream grpc.ClientStreamingClient[Chunk, UploadResult]) (UploadResult, error) {
//	        for _, chunk := range chunks {
//	            if err := stream.Send(chunk); err != nil {
//	                return UploadResult{}, err
//	            }
//	        }
//	        resp, err := stream.CloseAndRecv()
//	        if err != nil {
//	            return UploadResult{}, err
//	        }
//	        return *resp, nil
//	    },
//	)
//	if !t.NoErr("grpc", r.Err) { return }
//	t.Max("send", r.SendDuration, 500*time.Millisecond)
//	t.Max("response", r.ResponseLatency, 200*time.Millisecond)
//
// SendDuration includes stream open. ResponseLatency is 0 when CloseAndRecv
// was never reached (for example after a send error). TotalDuration may
// exceed SendDuration + ResponseLatency because author code between the
// last Send and CloseAndRecv is not attributed to either phase.
//
// # Agent evaluations
//
// Eval cases receive the same [T] but their recorded leaves are surfaced
// as `Metric`s on the wire, so [T.Score] is the primary tool. Authors
// bring their own scoring: a canned unigram scorer is available via
// [Rouge1F1] and LLM-judges are plain Go calls that feed a score into
// t.Score.
//
//	s := evals.MustNewAgentEvalSuite("example-agent-v1",
//	    evals.WithEnv("agent-runtime"),
//	)
//	s.MustCase("golden-summary", func(ctx context.Context, t *evals.T) {
//	    r := evals.Call(ctx, func(ctx context.Context) (*agentpb.Reply, error) {
//	        return clients.Agent.Chat(ctx, prompt)
//	    })
//	    if !t.NoErr("transport", r.Err) {
//	        return
//	    }
//	    t.Score("rouge-1", evals.Rouge1F1(r.Resp.GetText(), golden), 0.5, "vs golden")
//	})
//	if err := evals.RegisterEval(s); err != nil {
//	    panic(err)
//	}
//
// Agent evaluations can also be sourced lazily via [RegisterAgent] +
// [registry.AgentEvalProvider]; the [adk] subpackage provides a provider
// that discovers eval sets from a deployed ADK agent.
//
// # Load tests
//
// Load suites are shaped differently: cases are not func(ctx, *T) but a
// single [ResultTarget] the framework invokes many times under a resolved
// [Profile]. There is no [T]; the assertions are SLOs declared alongside
// the target. Use [TransportTarget] for transport-only RPCs or return
// [TargetResult] directly for semantic checks and stream timing. See
// [SLOLatencyP50], [SLOLatencyP95], [SLOLatencyP99], [SLOErrorRate],
// [SLOMinQPS], [SLOStreamTTFB], and [SLOMessagesPerSec].
//
// [WithLoadCaseTags], [WithLoadCaseData], and [WithLoadCaseDataProvider]
// configure per-case labels and payload rotation. [runner.WithAbortOnSLOFailure]
// cancels a case early when any declared SLO fails on partial metrics.
//
//	s := evals.MustNewLoadSuite("example-v1-load", evals.WithLoadEnv("example-v1"))
//	s.MustLoadCase("list-items",
//	    evals.TransportTarget(func(ctx context.Context) error {
//	        _, err := clients.Example.ListItems(ctx, &examplepb.ListItemsRequest{PageSize: 5})
//	        return err
//	    }),
//	    []evals.SLO{
//	        evals.SLOLatencyP99(500 * time.Millisecond),
//	        evals.SLOErrorRate(0.01),
//	    },
//	)
//	if err := evals.RegisterLoad(s); err != nil {
//	    panic(err)
//	}
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
//   - [WithContext](fn) — install a [ContextDecorator] that transforms
//     the outgoing context handed to setup, teardown, and every case in
//     the suite. This is the framework's only auth-adjacent surface;
//     callers stamp caller identity, auth headers, tracing state, or any
//     other request-scoped values here.
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
//	env.MustRegister("example-v1",
//	    env.WithSetup(seedExample),
//	    env.WithTeardown(cleanupExample),
//	)
//
// Environments are activated once per RunTest/RunEval/RunLoad LRO, not
// per suite. This lets several suites share expensive setup (seeding a
// database, warming a cache) without paying the cost repeatedly.
//
// # Context and authentication
//
// The framework never attaches auth to outgoing calls itself. Callers
// wire whatever auth they use — bearer tokens, oauth2, IAM identity
// headers, mTLS, etc. — by supplying a [ContextDecorator] via
// [WithContext] at the suite level, or via runner-level context
// decoration for a cross-suite default. The decorator receives the
// caller-supplied ctx and returns a derived ctx used for every hook and
// case body in the suite. Whatever the decorator attaches (metadata,
// tokens, values) propagates through Go's normal context inheritance to
// every outbound call the case body makes.
//
// Case authors always retain the escape hatch of further decorating the
// ctx they receive inside the case body. The ctx handed to a case is
// always a descendant of the ctx passed to the LRO, so deadlines,
// cancellation, and existing values are preserved.
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
