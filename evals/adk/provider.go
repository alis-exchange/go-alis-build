package adk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.alis.build/adk/launchers/evals/evaluation/models"
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

// filterPath is a parsed case filter ("suite" or "suite.case").
type filterPath struct {
	suite    string
	caseName string
}

func parseFilterPaths(paths []string) ([]filterPath, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	out := make([]filterPath, len(paths))
	for i, path := range paths {
		parsed, err := parseFilterPath(path)
		if err != nil {
			return nil, err
		}
		out[i] = parsed
	}
	return out, nil
}

func parseFilterPath(path string) (filterPath, error) {
	if path == "" {
		return filterPath{}, fmt.Errorf("adk: invalid filter path %q: empty filter path", path)
	}
	if strings.Count(path, ".") > 1 {
		return filterPath{}, fmt.Errorf("adk: invalid filter path %q: at most one '.' allowed", path)
	}
	suiteName, caseName, hasCase := strings.Cut(path, ".")
	if suiteName == "" || hasCase && caseName == "" {
		return filterPath{}, fmt.Errorf("adk: invalid filter path %q", path)
	}
	return filterPath{suite: suiteName, caseName: caseName}, nil
}

// matchFilters returns whether setID is mentioned in filters, whether all cases
// in the set should run, and which case IDs to run when wantAll is false.
func matchFilters(parsed []filterPath, setID string) (wantAll bool, caseIDs []string, mentioned bool) {
	if len(parsed) == 0 {
		return true, nil, true
	}

	seen := make(map[string]struct{})
	for _, f := range parsed {
		if f.suite != setID {
			continue
		}
		mentioned = true
		if f.caseName == "" {
			return true, nil, true
		}
		if _, ok := seen[f.caseName]; ok {
			continue
		}
		seen[f.caseName] = struct{}{}
		caseIDs = append(caseIDs, f.caseName)
	}
	if !mentioned {
		return false, nil, false
	}
	return false, caseIDs, true
}

// judgeMetricNames is the exact set of metrics backed by an LLM judge.
var judgeMetricNames = map[string]struct{}{
	models.MetricFinalResponseMatchV2:                    {},
	models.MetricRubricBasedFinalResponseQualityV1:       {},
	models.MetricRubricBasedToolUseQualityV1:             {},
	models.MetricRubricBasedMultiTurnTrajectoryQualityV1: {},
	models.MetricHallucinationsV1:                        {},
	models.MetricPerTurnUserSimulatorQualityV1:           {},
}

func isJudgeMetric(name string) bool {
	_, ok := judgeMetricNames[name]
	return ok
}

// probeJudgeModel returns the first configured judge model in declaration order.
func probeJudgeModel(metrics []models.EvalMetric) string {
	for _, m := range metrics {
		if v, ok := m.Criterion.AsLlmJudge(); ok {
			if v.JudgeModelOptions.JudgeModel != "" {
				return v.JudgeModelOptions.JudgeModel
			}
			continue
		}
		if v, ok := m.Criterion.AsRubrics(); ok {
			if v.JudgeModelOptions.JudgeModel != "" {
				return v.JudgeModelOptions.JudgeModel
			}
			continue
		}
		if v, ok := m.Criterion.AsHallucinations(); ok {
			if v.JudgeModelOptions.JudgeModel != "" {
				return v.JudgeModelOptions.JudgeModel
			}
		}
	}
	return ""
}
