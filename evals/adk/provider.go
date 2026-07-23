package adk

import (
	"context"
	"time"

	evalspb "go.alis.build/common/alis/evals/v1"
)

// clientFactory constructs a [Client] for one provider run; overridden in tests
// via [WithClientFactory] to inject mocks or authenticated transports.
type clientFactory func(ctx context.Context, baseURL, pathPrefix string) (Client, error)

// Provider runs agent evaluations by discovering eval sets from a deployed ADK agent.
type Provider struct {
	// agent holds the deployed agent URL, app name, metrics, and filters.
	agent Agent
	// newClient is the client constructor; defaults to unauthenticated [NewHTTPClient].
	newClient clientFactory
}

// ProviderOption configures a Provider.
type ProviderOption func(*Provider)

// ProviderResult is one ADK eval set materialized as protobuf-native results.
type ProviderResult struct {
	SuiteName string
	StartTime time.Time
	EndTime   time.Time
	Results   *evalspb.AgentEvalResults
}

// WithClientFactory overrides the default HTTP client factory (for tests).
func WithClientFactory(fn clientFactory) ProviderOption {
	return func(p *Provider) {
		p.newClient = fn
	}
}

// NewProvider constructs a lazy agent eval provider for the given agent config.
//
// The default client factory returns an unauthenticated [HTTPClient]
// (see [NewHTTPClient]) — suitable for local or already-authenticated
// endpoints. Consumers whose ADK sublauncher requires auth supply a
// custom factory via [WithClientFactory] that constructs the client with
// their own [WithTransport].
func NewProvider(agent Agent, opts ...ProviderOption) *Provider {
	p := &Provider{
		agent: agent,
		newClient: func(_ context.Context, baseURL, pathPrefix string) (Client, error) {
			return NewHTTPClient(baseURL, WithPathPrefix(pathPrefix)), nil
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Run discovers eval sets, runs filtered cases, and returns protobuf-native results.
func (p *Provider) Run(ctx context.Context, filters []string) ([]ProviderResult, error) {
	if p == nil {
		return nil, ErrNilProvider{}
	}
	if p.agent.BaseURL == "" || p.agent.AppName == "" {
		return nil, ErrMissingProviderConfig{}
	}

	client, err := p.newClient(ctx, p.agent.BaseURL, p.agent.pathPrefix())
	if err != nil {
		return nil, err
	}

	setIDs, err := client.ListEvalSets(ctx, p.agent.AppName)
	if err != nil {
		return nil, ErrListEvalSets{Err: err}
	}

	parsed, err := parseFilterPaths(filters)
	if err != nil {
		return nil, err
	}

	var out []ProviderResult
	for _, setID := range setIDs {
		if p.agent.IncludeEvalSet != nil && !p.agent.IncludeEvalSet(setID) {
			continue
		}

		wantAll, caseIDs, mentioned := matchFilters(parsed, setID)
		if len(parsed) > 0 && !mentioned {
			continue
		}

		params := RunEvalParams{
			AppName:   p.agent.AppName,
			EvalSetID: setID,
			Metrics:   p.agent.MetricsFor(setID),
		}
		if !wantAll {
			params.EvalCaseIDs = caseIDs
		}

		start := time.Now()
		results, err := client.RunEval(ctx, params)
		end := time.Now()
		if err != nil {
			return nil, ErrRunEval{SetID: setID, Err: err}
		}

		// Resolve judge provenance for this suite. Agent.JudgeModel is authoritative;
		// probeJudgeModel is a best-effort fallback that walks caller-supplied metric
		// criteria for this set.
		judgeModel := p.agent.JudgeModel
		if judgeModel == "" {
			judgeModel = probeJudgeModel(params.Metrics)
		}
		judge := SynthesizeJudgeContext(results, judgeModel, p.agent.JudgeModelVersion)
		out = append(out, ProviderResult{
			SuiteName: setID,
			StartTime: start,
			EndTime:   end,
			Results:   AgentEvalResultsFromRunEvalResults(setID, results, repeatedDuration(len(results), end.Sub(start)), judge),
		})
	}

	return out, nil
}

func repeatedDuration(n int, total time.Duration) []time.Duration {
	if n == 0 {
		return nil
	}
	each := total / time.Duration(n)
	out := make([]time.Duration, n)
	for i := range out {
		out[i] = each
	}
	return out
}
