package adk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.alis.build/adk/launchers/evals/evaluation/models"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
	"golang.org/x/oauth2"
)

// Client runs agent evaluations via the ADK evals HTTP sublauncher.
type Client interface {
	RunEval(ctx context.Context, params RunEvalParams) ([]models.RunEvalResult, error)
	ListEvalSets(ctx context.Context, appName string) ([]string, error)
}

// RunEvalParams configures one run_eval request.
type RunEvalParams struct {
	BaseURL     string
	PathPrefix  string // default "/api"
	AppName     string
	EvalSetID   string
	EvalCaseIDs []string
	Metrics     []models.EvalMetric
}

type runEvalRequest struct {
	EvalCaseIDs []string            `json:"eval_case_ids,omitempty"`
	EvalMetrics []models.EvalMetric `json:"eval_metrics"`
}

type runEvalResponse struct {
	RunEvalResults []models.RunEvalResult `json:"runEvalResults"`
}

// HTTPClient calls the ADK evals launcher over HTTP with Cloud Run auth headers.
type HTTPClient struct {
	httpClient *http.Client
	baseURL    string
	pathPrefix string
}

// NewHTTPClient constructs a client that mints a Cloud Run ID token for baseURL.
func NewHTTPClient(ctx context.Context, baseURL string) (*HTTPClient, error) {
	audience, err := AudienceFromBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	tokenSource, err := idtoken.NewTokenSource(ctx, audience, option.WithAudiences(audience))
	if err != nil {
		return nil, fmt.Errorf("adk client: token source: %w", err)
	}
	return NewHTTPClientWithTokenSource(baseURL, "/api", tokenSource), nil
}

// NewHTTPClientWithTokenSource constructs a client with a preconfigured token source.
func NewHTTPClientWithTokenSource(baseURL, pathPrefix string, tokenSource oauth2.TokenSource) *HTTPClient {
	if pathPrefix == "" {
		pathPrefix = "/api"
	}
	return &HTTPClient{
		baseURL:    strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		pathPrefix: pathPrefix,
		httpClient: &http.Client{
			Transport: &Transport{
				Base:        http.DefaultTransport,
				TokenSource: tokenSource,
			},
			Timeout: 10 * time.Minute,
		},
	}
}

// WithPathPrefix overrides the default "/api" path prefix.
func (c *HTTPClient) WithPathPrefix(prefix string) *HTTPClient {
	if c == nil {
		return nil
	}
	if prefix == "" {
		c.pathPrefix = "/api"
	} else {
		c.pathPrefix = prefix
	}
	return c
}

// RunEval POSTs to .../eval_sets/{evalSetId}/run_eval and decodes results.
func (c *HTTPClient) RunEval(ctx context.Context, params RunEvalParams) ([]models.RunEvalResult, error) {
	if c == nil {
		return nil, fmt.Errorf("adk client: nil client")
	}
	baseURL := params.BaseURL
	if baseURL == "" {
		baseURL = c.baseURL
	}
	pathPrefix := params.PathPrefix
	if pathPrefix == "" {
		pathPrefix = c.pathPrefix
	}
	if params.AppName == "" || params.EvalSetID == "" {
		return nil, fmt.Errorf("adk client: app name and eval set id are required")
	}

	body, err := json.Marshal(runEvalRequest{
		EvalCaseIDs: params.EvalCaseIDs,
		EvalMetrics: params.Metrics,
	})
	if err != nil {
		return nil, fmt.Errorf("adk client: encode request: %w", err)
	}

	url := fmt.Sprintf("%s%s/dev/apps/%s/eval_sets/%s/run_eval",
		strings.TrimSuffix(baseURL, "/"),
		pathPrefix,
		params.AppName,
		params.EvalSetID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("adk client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentUnreachable, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("adk client: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrRunEvalFailed, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	results, err := decodeRunEvalResults(raw)
	if err != nil {
		return nil, fmt.Errorf("adk client: decode response: %w", err)
	}
	return results, nil
}

// ListEvalSets GETs .../eval_sets and decodes eval set ids for an app.
func (c *HTTPClient) ListEvalSets(ctx context.Context, appName string) ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("adk client: nil client")
	}
	if appName == "" {
		return nil, fmt.Errorf("adk client: app name is required")
	}

	url := fmt.Sprintf("%s%s/dev/apps/%s/eval_sets",
		c.baseURL,
		c.pathPrefix,
		appName,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("adk client: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentUnreachable, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("adk client: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrRunEvalFailed, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var ids []string
	if err := json.Unmarshal(trimmed, &ids); err != nil {
		return nil, fmt.Errorf("adk client: decode eval sets: %w", err)
	}
	return ids, nil
}

func decodeRunEvalResults(raw []byte) ([]models.RunEvalResult, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var results []models.RunEvalResult
		if err := json.Unmarshal(trimmed, &results); err != nil {
			return nil, err
		}
		return results, nil
	}
	var wrapped runEvalResponse
	if err := json.Unmarshal(trimmed, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.RunEvalResults, nil
}
