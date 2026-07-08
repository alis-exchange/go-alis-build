package adk

import (
	"context"
	"fmt"
	"time"

	"go.alis.build/evals/execution"
	"go.alis.build/evals/suite"
)

type clientFactory func(ctx context.Context, baseURL, pathPrefix string) (Client, error)

// Provider runs agent evaluations by discovering eval sets from a deployed ADK agent.
type Provider struct {
	agent     Agent
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
func NewProvider(agent Agent, opts ...ProviderOption) *Provider {
	p := &Provider{
		agent: agent,
		newClient: func(ctx context.Context, baseURL, pathPrefix string) (Client, error) {
			c, err := NewHTTPClient(ctx, baseURL)
			if err != nil {
				return nil, err
			}
			return c.WithPathPrefix(pathPrefix), nil
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
		return nil, fmt.Errorf("adk provider: nil provider")
	}
	if p.agent.BaseURL == "" || p.agent.AppName == "" {
		return nil, fmt.Errorf("adk provider: base URL and app name are required")
	}

	client, err := p.newClient(ctx, p.agent.BaseURL, p.agent.pathPrefix())
	if err != nil {
		return nil, err
	}

	setIDs, err := client.ListEvalSets(ctx, p.agent.AppName)
	if err != nil {
		return nil, fmt.Errorf("list eval sets: %w", err)
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
			return nil, fmt.Errorf("run_eval %s: %w", setID, err)
		}

		each := end.Sub(start) / time.Duration(max(len(results), 1))
		cases := make([]execution.CaseResult, 0, len(results))
		for _, r := range results {
			cr := CaseFromRunEvalResult(r, each)
			cr.Name = suite.QualifiedName(setID, r.EvalID)
			cases = append(cases, *cr)
		}

		out = append(out, execution.SuiteResult{
			SuiteName: setID,
			Cases:     cases,
			StartTime: start,
			EndTime:   end,
		})
	}

	return out, nil
}
