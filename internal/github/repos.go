package github

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// GetRepo fetches one repository, for example owner "golang", repo "go".
// https://docs.github.com/en/rest/repos/repos#get-a-repository
func (c *Client) GetRepo(ctx context.Context, owner, repo string) (*Repository, error) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return nil, errors.New("owner and repo must not be empty")
	}

	var r Repository
	path := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	if _, err := c.get(ctx, path, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListUserReposOptions controls sorting and paging for ListUserRepos.
// Empty or zero fields are left out, so GitHub's defaults apply.
type ListUserReposOptions struct {
	Sort      string // "created", "updated", "pushed" or "full_name" (default)
	Direction string // "asc" or "desc"
	PerPage   int    // 1-100, GitHub default 30
	Page      int    // 1-based page number
}

// ListUserRepos lists public repositories owned by a user or organization.
// hasMore is true when GitHub says there is another page.
// https://docs.github.com/en/rest/repos/repos#list-repositories-for-a-user
func (c *Client) ListUserRepos(ctx context.Context, username string, opts ListUserReposOptions) (repos []Repository, hasMore bool, err error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, false, errors.New("username must not be empty")
	}

	query := url.Values{}
	if opts.Sort != "" {
		query.Set("sort", opts.Sort)
	}
	if opts.Direction != "" {
		query.Set("direction", opts.Direction)
	}
	setPaging(query, opts.PerPage, opts.Page)

	header, err := c.get(ctx, "/users/"+url.PathEscape(username)+"/repos", query, &repos)
	if err != nil {
		return nil, false, err
	}
	return repos, hasNextPage(header), nil
}
