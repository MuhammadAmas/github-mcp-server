package github

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// SearchReposOptions controls sorting and paging for SearchRepositories.
// Empty or zero fields are left out, so GitHub's defaults apply.
type SearchReposOptions struct {
	Sort    string // "stars", "forks", "help-wanted-issues" or "updated"; empty = best match
	Order   string // "desc" (default) or "asc"; only used when Sort is set
	PerPage int    // 1-100, GitHub default 30
	Page    int    // 1-based page number
}

// SearchRepositories searches public repositories. The query uses GitHub's
// search syntax, e.g. "mcp language:go stars:>100".
//
// Search has its own, stricter rate limit: 10 requests per minute without
// a token, 30 with one.
// https://docs.github.com/en/rest/search/search#search-repositories
func (c *Client) SearchRepositories(ctx context.Context, query string, opts SearchReposOptions) (*SearchRepositoriesResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query must not be empty")
	}

	params := url.Values{}
	params.Set("q", query) // Encode() escapes spaces, ':' and '>' for us
	if opts.Sort != "" {
		params.Set("sort", opts.Sort)
	}
	if opts.Order != "" {
		params.Set("order", opts.Order)
	}
	setPaging(params, opts.PerPage, opts.Page)

	var result SearchRepositoriesResult
	if _, err := c.get(ctx, "/search/repositories", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
