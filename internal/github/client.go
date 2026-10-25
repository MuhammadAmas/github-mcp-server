// Package github is a small client for the GitHub REST API.
//
// It knows nothing about MCP: the tool handlers in internal/tools call it,
// which keeps HTTP details and MCP details in separate places.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the public GitHub REST API.
	DefaultBaseURL = "https://api.github.com"
	// DefaultTimeout bounds every request, including reading the response.
	DefaultTimeout = 10 * time.Second

	apiVersion = "2022-11-28"
	userAgent  = "github-mcp-server/0.1"
)

// Options configures a Client. The zero value is valid and gives you an
// unauthenticated client for api.github.com with a 10s timeout.
type Options struct {
	// BaseURL overrides the API address. Tests point this at an httptest server.
	BaseURL string
	// Token is an optional GitHub token. Empty means unauthenticated requests
	// (60 requests/hour). Never hardcode it; read it from the environment.
	Token string
	// Timeout overrides DefaultTimeout.
	Timeout time.Duration
}

// Client calls the GitHub REST API. Create one with NewClient and share it:
// it is safe to use from many goroutines at once.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient returns a Client configured by opts.
func NewClient(opts Options) *Client {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      opts.Token,
	}
}

// get sends a GET request to path (plus optional query parameters) and
// decodes a successful JSON response into out, which must be a pointer.
// It returns the response headers so callers can read pagination links.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) (http.Header, error) {
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	// NewRequestWithContext ties the request to ctx: if ctx is cancelled
	// (client gave up, Ctrl+C, deadline), the HTTP call is aborted.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("User-Agent", userAgent) // GitHub rejects requests without one
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network failure, timeout, or cancelled context. The error text from
		// net/http includes the URL but never the Authorization header.
		return nil, fmt.Errorf("calling GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, newAPIError(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("decoding GitHub response: %w", err)
	}
	return resp.Header, nil
}

// setPaging adds per_page and page to query when they are set (> 0).
// GitHub uses its own defaults for anything we leave out.
func setPaging(query url.Values, perPage, page int) {
	if perPage > 0 {
		query.Set("per_page", strconv.Itoa(perPage))
	}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
}

// hasNextPage reports whether GitHub's Link header points to another page.
// Example header: <https://api.github.com/...&page=2>; rel="next", <...>; rel="last"
func hasNextPage(header http.Header) bool {
	return strings.Contains(header.Get("Link"), `rel="next"`)
}
