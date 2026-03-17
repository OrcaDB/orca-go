package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL       = "https://api.orcadb.ai/"
	envAPIKey            = "ORCA_API_KEY"
	envAPIURL            = "ORCA_API_URL"
	defaultMaxRetries    = 3
	defaultBaseDelay     = 500 * time.Millisecond
	userAgent            = "orca-go/0.1.0"
	maxResponseBodyBytes = 10 * 1024 * 1024 // 10 MB
)

// Client is an HTTP client for the Orca API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	maxRetries int
	baseDelay  time.Duration
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithAPIKey sets the API key. Overrides the ORCA_API_KEY environment variable.
func WithAPIKey(key string) ClientOption {
	return func(c *Client) { c.apiKey = key }
}

// WithBaseURL sets the API base URL. Overrides the ORCA_API_URL environment variable.
func WithBaseURL(u string) ClientOption {
	return func(c *Client) { c.baseURL = u }
}

// WithHTTPClient sets a custom *http.Client for all requests.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithRetries sets the maximum number of retries for transient errors
// (HTTP 429, 500, 502, 503, 504). Set to 0 to disable retries. Default is 3.
func WithRetries(n int) ClientOption {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxRetries = n
	}
}

// NewClient creates a new Orca API client.
// It reads ORCA_API_KEY and ORCA_API_URL from environment variables as defaults,
// which can be overridden with WithAPIKey and WithBaseURL options.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    os.Getenv(envAPIURL),
		apiKey:     os.Getenv(envAPIKey),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		maxRetries: defaultMaxRetries,
		baseDelay:  defaultBaseDelay,
	}
	if c.baseURL == "" {
		c.baseURL = defaultBaseURL
	}
	for _, opt := range opts {
		opt(c)
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	return c
}

// IsHealthy checks whether the Orca API is reachable and healthy.
func (c *Client) IsHealthy(ctx context.Context) bool {
	data, err := c.doRequest(ctx, http.MethodGet, "/check/healthy", nil)
	if err != nil {
		return false
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if json.Unmarshal(data, &result) != nil {
		return false
	}
	return result.OK
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
	}

	fullURL := c.baseURL + path

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			base := c.baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
			delay := time.Duration(float64(base) * (0.5 + rand.Float64()*0.5))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Api-Key", c.apiKey)
		req.Header.Set("User-Agent", userAgent)
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response body: %w", err)
			continue
		}

		if isRetryable(resp.StatusCode) && attempt < c.maxRetries {
			lastErr = parseAPIError(resp.StatusCode, respBody)
			continue
		}

		if resp.StatusCode >= 400 {
			return nil, parseAPIError(resp.StatusCode, respBody)
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

func isRetryable(code int) bool {
	switch code {
	case 429, 500, 502, 503, 504:
		return true
	}
	return false
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

func (c *Client) post(ctx context.Context, path string, body any) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, body)
}

func (c *Client) patch(ctx context.Context, path string, body any) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPatch, path, body)
}
