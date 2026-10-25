package github

import "time"

// User is the subset of GitHub's user object that we care about.
// Full response: https://docs.github.com/en/rest/users/users#get-a-user
//
// Fields GitHub sends as null (for example "email") are decoded as "".
// Fields we don't list here (node_id, gists_url, ...) are ignored.
type User struct {
	Login       string    `json:"login"`
	ID          int64     `json:"id"`
	Type        string    `json:"type"` // "User" or "Organization"
	Name        string    `json:"name"`
	Company     string    `json:"company"`
	Blog        string    `json:"blog"`
	Location    string    `json:"location"`
	Email       string    `json:"email"`
	Bio         string    `json:"bio"`
	PublicRepos int       `json:"public_repos"`
	PublicGists int       `json:"public_gists"`
	Followers   int       `json:"followers"`
	Following   int       `json:"following"`
	HTMLURL     string    `json:"html_url"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Repository is the subset of GitHub's repository object that we care about.
// The same shape is returned by "get a repository", "list repositories for
// a user" and repository search.
// https://docs.github.com/en/rest/repos/repos#get-a-repository
type Repository struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`      // e.g. "go"
	FullName        string   `json:"full_name"` // e.g. "golang/go"
	Owner           Owner    `json:"owner"`
	Description     string   `json:"description"`
	HTMLURL         string   `json:"html_url"`
	Homepage        string   `json:"homepage"`
	Language        string   `json:"language"`
	Topics          []string `json:"topics"`
	License         *License `json:"license"` // pointer: GitHub sends null when there is no license
	Fork            bool     `json:"fork"`
	Archived        bool     `json:"archived"`
	Visibility      string   `json:"visibility"`
	DefaultBranch   string   `json:"default_branch"`
	StargazersCount int      `json:"stargazers_count"`
	ForksCount      int      `json:"forks_count"`
	OpenIssuesCount int      `json:"open_issues_count"` // issues + pull requests
	// SubscribersCount is the real "watchers" number. GitHub only includes it
	// in "get a repository", not in list or search results.
	SubscribersCount int       `json:"subscribers_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	PushedAt         time.Time `json:"pushed_at"`
}

// Owner is the user or organization that owns a repository.
type Owner struct {
	Login string `json:"login"`
	Type  string `json:"type"` // "User" or "Organization"
}

// License describes a repository's detected license.
type License struct {
	SPDXID string `json:"spdx_id"` // e.g. "MIT", "BSD-3-Clause"
	Name   string `json:"name"`
}

// SearchRepositoriesResult is the response of the repository search API.
// https://docs.github.com/en/rest/search/search#search-repositories
type SearchRepositoriesResult struct {
	TotalCount        int          `json:"total_count"`
	IncompleteResults bool         `json:"incomplete_results"` // true if GitHub timed out and returned partial results
	Items             []Repository `json:"items"`
}
