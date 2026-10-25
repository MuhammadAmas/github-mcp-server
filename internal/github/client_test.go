package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient starts a fake GitHub server that answers with handler and
// returns a Client pointed at it. The server is shut down when the test ends.
func newTestClient(t *testing.T, opts Options, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	opts.BaseURL = srv.URL
	return NewClient(opts)
}

// mustNotCall is a handler that fails the test if any request reaches it.
func mustNotCall(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL)
	}
}

func TestGetUser_Success(t *testing.T) {
	client := newTestClient(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/users/octocat" {
			t.Errorf("path = %s, want /users/octocat", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q", got)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != apiVersion {
			t.Errorf("X-GitHub-Api-Version = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != userAgent {
			t.Errorf("User-Agent = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want none without a token", got)
		}
		w.Write([]byte(`{"login":"octocat","name":"The Octocat","email":null,"public_repos":8,"created_at":"2011-01-25T18:44:36Z"}`))
	})

	user, err := client.GetUser(context.Background(), "  octocat  ")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Login != "octocat" || user.Name != "The Octocat" || user.PublicRepos != 8 {
		t.Errorf("unexpected user: %+v", user)
	}
	if user.Email != "" {
		t.Errorf("Email = %q, want empty for JSON null", user.Email)
	}
	if want := time.Date(2011, 1, 25, 18, 44, 36, 0, time.UTC); !user.CreatedAt.Equal(want) {
		t.Errorf("CreatedAt = %v, want %v", user.CreatedAt, want)
	}
}

func TestGetUser_SendsToken(t *testing.T) {
	client := newTestClient(t, Options{Token: "test-token"}, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		w.Write([]byte(`{"login":"octocat"}`))
	})
	if _, err := client.GetUser(context.Background(), "octocat"); err != nil {
		t.Fatalf("GetUser: %v", err)
	}
}

func TestGetUser_EmptyUsernameMakesNoRequest(t *testing.T) {
	client := newTestClient(t, Options{}, mustNotCall(t))
	if _, err := client.GetUser(context.Background(), "   "); err == nil {
		t.Fatal("expected an error for an empty username")
	}
}

func TestGetUser_EscapesPath(t *testing.T) {
	client := newTestClient(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/users/a%2Fb" {
			t.Errorf("escaped path = %s, want /users/a%%2Fb", got)
		}
		w.WriteHeader(http.StatusNotFound)
	})
	_, _ = client.GetUser(context.Background(), "a/b")
}

// TestAPIErrors is a table-driven test: each row is one fake GitHub response
// and what we expect the client to make of it.
func TestAPIErrors(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		headers     map[string]string
		body        string
		notFound    bool
		rateLimited bool
		wantInError string
	}{
		{
			name: "404 not found", status: 404,
			body:     `{"message":"Not Found"}`,
			notFound: true, wantInError: "Not Found",
		},
		{
			name: "403 primary rate limit", status: 403,
			headers:     map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1792091524"},
			body:        `{"message":"API rate limit exceeded"}`,
			rateLimited: true, wantInError: "resets at 2026-10-15T19:12:04Z",
		},
		{
			name: "403 secondary rate limit", status: 403,
			headers:     map[string]string{"Retry-After": "60"},
			rateLimited: true, wantInError: "retry after 1m0s",
		},
		{
			name: "403 forbidden", status: 403,
			headers:     map[string]string{"X-RateLimit-Remaining": "59"},
			body:        `{"message":"Resource not accessible"}`,
			wantInError: "Resource not accessible",
		},
		{
			name: "429 too many requests", status: 429,
			headers:     map[string]string{"Retry-After": "30"},
			rateLimited: true, wantInError: "retry after 30s",
		},
		{
			name: "422 with details", status: 422,
			body:        `{"message":"Validation Failed","errors":[{"resource":"Search","field":"q","code":"missing"},"plain text reason"]}`,
			wantInError: "Validation Failed: q missing; plain text reason",
		},
		{
			name: "500 with HTML body", status: 500,
			body:        `<html>oops</html>`,
			wantInError: "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			})

			_, err := client.GetUser(context.Background(), "octocat")

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %v (%T), want *APIError", err, err)
			}
			if apiErr.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.status)
			}
			if got := IsNotFound(err); got != tt.notFound {
				t.Errorf("IsNotFound = %v, want %v", got, tt.notFound)
			}
			if got := IsRateLimited(err); got != tt.rateLimited {
				t.Errorf("IsRateLimited = %v, want %v", got, tt.rateLimited)
			}
			if !strings.Contains(err.Error(), tt.wantInError) {
				t.Errorf("error %q does not contain %q", err, tt.wantInError)
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	client := newTestClient(t, Options{Timeout: 50 * time.Millisecond}, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	})
	_, err := client.GetUser(context.Background(), "octocat")
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Errorf("a timeout should not be an *APIError, got %v", apiErr)
	}
}

func TestCancelledContext(t *testing.T) {
	client := newTestClient(t, Options{}, mustNotCall(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call even starts

	_, err := client.GetUser(ctx, "octocat")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want one wrapping context.Canceled", err)
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(Options{})
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.httpClient.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", c.httpClient.Timeout, DefaultTimeout)
	}

	c = NewClient(Options{BaseURL: "https://example.test/api/v3/"})
	if c.baseURL != "https://example.test/api/v3" {
		t.Errorf("trailing slash not trimmed: %q", c.baseURL)
	}
}
