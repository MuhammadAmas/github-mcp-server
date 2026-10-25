package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIError is returned when GitHub answers with a non-2xx status code.
// Callers can inspect it with errors.As, or use the IsNotFound and
// IsRateLimited helpers below.
type APIError struct {
	StatusCode int      // HTTP status, e.g. 404
	Message    string   // GitHub's "message" field, e.g. "Not Found"
	Details    []string // extra reasons from GitHub's "errors" list (common with 422)

	// RateLimited is true for 429, and for 403 responses caused by rate limits.
	RateLimited bool
	// ResetAt is when the primary rate limit resets (zero if unknown).
	ResetAt time.Time
	// RetryAfter is how long GitHub asked us to wait (zero if not sent).
	RetryAfter time.Duration
}

// Error makes *APIError satisfy Go's built-in error interface.
func (e *APIError) Error() string {
	switch {
	case e.RateLimited && e.RetryAfter > 0:
		return fmt.Sprintf("GitHub rate limit exceeded (HTTP %d): retry after %s", e.StatusCode, e.RetryAfter)
	case e.RateLimited && !e.ResetAt.IsZero():
		return fmt.Sprintf("GitHub rate limit exceeded (HTTP %d): resets at %s", e.StatusCode, e.ResetAt.Format(time.RFC3339))
	case e.RateLimited:
		return fmt.Sprintf("GitHub rate limit exceeded (HTTP %d)", e.StatusCode)
	case e.Message != "" && len(e.Details) > 0:
		return fmt.Sprintf("GitHub API error (HTTP %d): %s: %s", e.StatusCode, e.Message, strings.Join(e.Details, "; "))
	case e.Message != "":
		return fmt.Sprintf("GitHub API error (HTTP %d): %s", e.StatusCode, e.Message)
	default:
		return fmt.Sprintf("GitHub API error (HTTP %d)", e.StatusCode)
	}
}

// IsNotFound reports whether err is (or wraps) a GitHub 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// IsRateLimited reports whether err is (or wraps) a GitHub rate-limit error.
func IsRateLimited(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.RateLimited
}

// maxErrorBody caps how much of an error response we read into memory.
const maxErrorBody = 64 << 10 // 64 KiB

// newAPIError builds an *APIError from a non-2xx response.
// The caller still owns resp.Body and must close it.
func newAPIError(resp *http.Response) *APIError {
	apiErr := &APIError{StatusCode: resp.StatusCode}

	// GitHub error bodies look like
	//   {"message": "Validation Failed", "errors": [{"field": "q", "code": "missing"}]}
	// If the body isn't valid JSON we simply keep Message empty.
	var body struct {
		Message string            `json:"message"`
		Errors  []json.RawMessage `json:"errors"` // items vary in shape, so decode each one below
	}
	if data, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody)); err == nil {
		_ = json.Unmarshal(data, &body)
	}
	apiErr.Message = body.Message
	for _, raw := range body.Errors {
		if detail := errorDetail(raw); detail != "" {
			apiErr.Details = append(apiErr.Details, detail)
		}
	}

	// Rate limits: https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
	// - 429 is always a rate limit.
	// - 403 is a rate limit when X-RateLimit-Remaining is "0" (primary limit)
	//   or when Retry-After is present (secondary limit). Otherwise it is a
	//   normal "forbidden" error.
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	retryAfter := resp.Header.Get("Retry-After")
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		apiErr.RateLimited = true
	case http.StatusForbidden:
		apiErr.RateLimited = remaining == "0" || retryAfter != ""
	}

	if secs, err := strconv.Atoi(retryAfter); err == nil && secs > 0 {
		apiErr.RetryAfter = time.Duration(secs) * time.Second
	}
	if unix, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil && unix > 0 {
		apiErr.ResetAt = time.Unix(unix, 0).UTC()
	}

	return apiErr
}

// errorDetail turns one item of GitHub's "errors" list into text.
// Items are usually objects ({"message": ...} or {"field": ..., "code": ...})
// but can also be plain strings.
func errorDetail(raw json.RawMessage) string {
	var item struct {
		Message string `json:"message"`
		Field   string `json:"field"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(raw, &item); err == nil {
		switch {
		case item.Message != "":
			return item.Message
		case item.Field != "" || item.Code != "":
			return strings.TrimSpace(item.Field + " " + item.Code)
		}
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return ""
}
