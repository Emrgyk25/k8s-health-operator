package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client interface {
	Query(ctx context.Context, query string) (*QueryResponse, error)
}

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

type QueryResponse struct {
	Status string          `json:"status"`
	Data   QueryData       `json:"data"`
	Error  string          `json:"error,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

type QueryData struct {
	ResultType string        `json:"resultType"`
	Result     []QueryResult `json:"result"`
}

type QueryResult struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"`
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *HTTPClient) Query(ctx context.Context, query string) (*QueryResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("prometheus base URL cannot be empty")
	}

	endpoint, err := url.Parse(c.baseURL + "/api/v1/query")
	if err != nil {
		return nil, fmt.Errorf("parse prometheus url: %w", err)
	}

	params := endpoint.Query()
	params.Set("query", query)
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create prometheus request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute prometheus query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("prometheus returned non-success status: %d", resp.StatusCode)
	}

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", result.Error)
	}

	return &result, nil
}
