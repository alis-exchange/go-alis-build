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

// defaultTimeout is the request timeout applied to newly constructed
// HTTPClients when the caller does not override via [WithTimeout]. It is
// generous by design: ADK eval runs are typically slow.
const defaultTimeout = 10 * time.Minute

// HTTPClient calls the ADK evals launcher over HTTP. It is transport-agnostic:
// callers plug in any authentication they need via [WithTransport].
type HTTPClient struct {
	httpClient *http.Client
	baseURL    string
	pathPrefix string
}

// HTTPClientOption configures an [HTTPClient] at construction time.
type HTTPClientOption func(*httpClientConfig)

type httpClientConfig struct {
	transport  http.RoundTripper
	timeout    time.Duration
	timeoutSet bool
	pathPrefix string
}

// WithTransport sets the [http.RoundTripper] used for outbound requests.
// Callers install their own authentication here (bearer tokens, oauth2,
// IAM identity headers, mTLS, etc.). A nil transport is equivalent to
// [http.DefaultTransport].
func WithTransport(rt http.RoundTripper) HTTPClientOption {
	return func(c *httpClientConfig) {
		c.transport = rt
	}
}

// WithTimeout overrides the default 10-minute request timeout applied by
// the underlying [http.Client]. A zero or negative duration disables the
// client-level timeout; callers can still cap individual requests via
// context deadlines.
func WithTimeout(d time.Duration) HTTPClientOption {
	return func(c *httpClientConfig) {
		c.timeout = d
		c.timeoutSet = true
	}
}

// WithPathPrefix overrides the default "/api" path prefix mounted by the
// ADK evals sublauncher. An empty prefix restores the default.
func WithPathPrefix(prefix string) HTTPClientOption {
	return func(c *httpClientConfig) {
		c.pathPrefix = prefix
	}
}

// NewHTTPClient constructs a client for the ADK evals sublauncher at
// baseURL. With no options the client uses [http.DefaultTransport] (no
// auth), a 10-minute request timeout suitable for long-running eval runs,
// and the "/api" path prefix.
//
// Plug in authentication via [WithTransport]; tune the request timeout
// via [WithTimeout]; override the path prefix via [WithPathPrefix].
func NewHTTPClient(baseURL string, opts ...HTTPClientOption) *HTTPClient {
	cfg := httpClientConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	transport := cfg.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	timeout := defaultTimeout
	if cfg.timeoutSet {
		if cfg.timeout > 0 {
			timeout = cfg.timeout
		} else {
			timeout = 0
		}
	}
	pathPrefix := cfg.pathPrefix
	if pathPrefix == "" {
		pathPrefix = defaultPathPrefix
	}
	return &HTTPClient{
		baseURL:    strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		pathPrefix: pathPrefix,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// RunEval POSTs to .../eval_sets/{evalSetId}/run_eval and decodes results.
func (c *HTTPClient) RunEval(ctx context.Context, params RunEvalParams) ([]models.RunEvalResult, error) {
	if c == nil {
		return nil, ErrNilClient{}
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
		return nil, ErrMissingAppNameEvalSetID{}
	}

	body, err := json.Marshal(runEvalRequest{
		EvalCaseIDs: params.EvalCaseIDs,
		EvalMetrics: params.Metrics,
	})
	if err != nil {
		return nil, ErrEncodeRequest{Err: err}
	}

	url := fmt.Sprintf("%s%s/dev/apps/%s/eval_sets/%s/run_eval",
		strings.TrimSuffix(baseURL, "/"),
		pathPrefix,
		params.AppName,
		params.EvalSetID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, ErrBuildRequest{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrAgentUnreachable{Cause: err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrReadResponse{Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrRunEvalFailed{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(raw)),
		}
	}

	results, err := decodeRunEvalResults(raw)
	if err != nil {
		return nil, ErrDecodeResponse{Err: err}
	}
	return results, nil
}

// ListEvalSets GETs .../eval_sets and decodes eval set ids for an app.
func (c *HTTPClient) ListEvalSets(ctx context.Context, appName string) ([]string, error) {
	if c == nil {
		return nil, ErrNilClient{}
	}
	if appName == "" {
		return nil, ErrMissingAppName{}
	}

	url := fmt.Sprintf("%s%s/dev/apps/%s/eval_sets",
		c.baseURL,
		c.pathPrefix,
		appName,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ErrBuildRequest{Err: err}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrAgentUnreachable{Cause: err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrReadResponse{Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrRunEvalFailed{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(raw)),
		}
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var ids []string
	if err := json.Unmarshal(trimmed, &ids); err != nil {
		return nil, ErrDecodeEvalSets{Err: err}
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
