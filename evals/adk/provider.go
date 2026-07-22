package adk

import (
	"context"
	"time"

	"go.alis.build/evals/execution"
	"go.alis.build/evals/suite"
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

// Run discovers eval sets, runs filtered cases, and returns suite results.
func (p *Provider) Run(ctx context.Context, filters []string) ([]execution.SuiteResult, error) {
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

	parsed, err := suite.ParseFilterPaths(filters)
	if err != nil {
		return nil, err
	}

	var out []execution.SuiteResult
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

		each := end.Sub(start) / time.Duration(max(len(results), 1))
		cases := make([]execution.CaseResult, 0, len(results))
		var suiteJudgeCalls int64
		for _, r := range results {
			cr := CaseFromRunEvalResult(setID, r, each)
			suiteJudgeCalls += cr.JudgeCallCount
			cases = append(cases, *cr)
		}

		// Resolve judge provenance for this suite. Agent.JudgeModel is
		// authoritative; probeJudgeModel is a best-effort fallback that
		// walks the caller-supplied metric criteria for this set. See
		// Agent.JudgeModel godoc for the caveats around heterogeneous
		// metric setups and the adk-python default asymmetry.
		judgeModel := p.agent.JudgeModel
		if judgeModel == "" {
			judgeModel = probeJudgeModel(params.Metrics)
		}

		out = append(out, execution.SuiteResult{
			SuiteName: setID,
			Cases:     cases,
			StartTime: start,
			EndTime:   end,
			Judge: execution.JudgeInfo{
				Model:        judgeModel,
				ModelVersion: p.agent.JudgeModelVersion,
			},
			JudgeCallCount: suiteJudgeCalls,
		})
	}

	return out, nil
}
