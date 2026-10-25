package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MuhammadAmas/github-mcp-server/internal/github"
)

// ---------- shared output types ----------

// RepoOutput is the full description of one repository (get_github_repo).
type RepoOutput struct {
	FullName      string   `json:"full_name" jsonschema:"owner/name, for example golang/go"`
	Owner         string   `json:"owner"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	URL           string   `json:"url"`
	Homepage      string   `json:"homepage,omitempty"`
	Language      string   `json:"language,omitempty" jsonschema:"primary programming language"`
	Topics        []string `json:"topics,omitempty"`
	License       string   `json:"license,omitempty" jsonschema:"SPDX license id such as MIT, or Other if GitHub could not classify it"`
	Stars         int      `json:"stars"`
	Forks         int      `json:"forks"`
	Watchers      int      `json:"watchers"`
	OpenIssues    int      `json:"open_issues" jsonschema:"open issues plus open pull requests"`
	DefaultBranch string   `json:"default_branch"`
	IsFork        bool     `json:"is_fork"`
	IsArchived    bool     `json:"is_archived"`
	CreatedAt     string   `json:"created_at"`
	PushedAt      string   `json:"pushed_at,omitempty" jsonschema:"time of the last push, RFC 3339"`
}

// RepoSummary is a shorter description used in lists and search results,
// so long lists don't flood the AI model's context window.
type RepoSummary struct {
	FullName    string   `json:"full_name"`
	Description string   `json:"description,omitempty"`
	Language    string   `json:"language,omitempty"`
	Topics      []string `json:"topics,omitempty"`
	Stars       int      `json:"stars"`
	Forks       int      `json:"forks"`
	IsFork      bool     `json:"is_fork"`
	IsArchived  bool     `json:"is_archived"`
	URL         string   `json:"url"`
	PushedAt    string   `json:"pushed_at,omitempty"`
}

func toRepoOutput(r *github.Repository) RepoOutput {
	out := RepoOutput{
		FullName:      r.FullName,
		Owner:         r.Owner.Login,
		Name:          r.Name,
		Description:   r.Description,
		URL:           r.HTMLURL,
		Homepage:      r.Homepage,
		Language:      r.Language,
		Topics:        r.Topics,
		Stars:         r.StargazersCount,
		Forks:         r.ForksCount,
		Watchers:      r.SubscribersCount,
		OpenIssues:    r.OpenIssuesCount,
		DefaultBranch: r.DefaultBranch,
		IsFork:        r.Fork,
		IsArchived:    r.Archived,
		CreatedAt:     formatTime(r.CreatedAt),
		PushedAt:      formatTime(r.PushedAt),
	}
	// License is a pointer because GitHub sends null for unlicensed repos.
	// Reading r.License.SPDXID when r.License is nil would crash, so check first.
	if r.License != nil {
		out.License = r.License.SPDXID
		// GitHub uses "NOASSERTION" when it found a license file it can't
		// classify; the name ("Other") is more useful to the reader then.
		if out.License == "" || out.License == "NOASSERTION" {
			out.License = r.License.Name
		}
	}
	return out
}

func toRepoSummaries(repos []github.Repository) []RepoSummary {
	// make with length 0 and capacity len(repos): an empty list encodes as [] (not null).
	out := make([]RepoSummary, 0, len(repos))
	for _, r := range repos {
		out = append(out, RepoSummary{
			FullName:    r.FullName,
			Description: r.Description,
			Language:    r.Language,
			Topics:      r.Topics,
			Stars:       r.StargazersCount,
			Forks:       r.ForksCount,
			IsFork:      r.Fork,
			IsArchived:  r.Archived,
			URL:         r.HTMLURL,
			PushedAt:    formatTime(r.PushedAt),
		})
	}
	return out
}

// ---------- get_github_repo ----------

// GetRepoInput is the argument schema for get_github_repo.
type GetRepoInput struct {
	Owner string `json:"owner" jsonschema:"user or organization that owns the repository, for example golang"`
	Repo  string `json:"repo" jsonschema:"repository name without the owner, for example go"`
}

// GetRepo is the logic of get_github_repo.
func GetRepo(ctx context.Context, gh *github.Client, in GetRepoInput) (RepoOutput, error) {
	owner := strings.TrimSpace(in.Owner)
	repo := strings.TrimSpace(in.Repo)
	if err := validateUsername(owner); err != nil {
		return RepoOutput{}, fmt.Errorf("owner: %w", err)
	}
	if err := validateRepoName(repo); err != nil {
		return RepoOutput{}, err
	}

	r, err := gh.GetRepo(ctx, owner, repo)
	if err != nil {
		return RepoOutput{}, githubError(err, fmt.Sprintf("repository %q", owner+"/"+repo))
	}
	return toRepoOutput(r), nil
}

// AddGetRepoTool registers get_github_repo.
func AddGetRepoTool(server *mcp.Server, gh *github.Client) {
	tool := &mcp.Tool{
		Name: "get_github_repo",
		Description: "Get details of one public GitHub repository: description, stars, forks, " +
			"watchers, open issues, primary language, topics, license and last push time.",
		Annotations: &mcp.ToolAnnotations{Title: "Get GitHub repository", ReadOnlyHint: true},
	}
	mcp.AddTool(server, tool, func(ctx context.Context, req *mcp.CallToolRequest, in GetRepoInput) (*mcp.CallToolResult, RepoOutput, error) {
		out, err := GetRepo(ctx, gh, in)
		return nil, out, err
	})
}

// ---------- list_user_repos ----------

// Allowed values, used both for the JSON Schema and for validation.
var (
	listSortValues      = []string{"created", "updated", "pushed", "full_name"}
	listDirectionValues = []string{"asc", "desc"}
)

const listDefaultPerPage = 30

// ListUserReposInput is the argument schema for list_user_repos.
// Fields with omitempty are optional.
type ListUserReposInput struct {
	Username  string `json:"username" jsonschema:"GitHub login of the user or organization, for example octocat"`
	Sort      string `json:"sort,omitempty" jsonschema:"sort field; default full_name"`
	Direction string `json:"direction,omitempty" jsonschema:"sort direction; default asc for full_name, otherwise desc"`
	PerPage   int    `json:"per_page,omitempty" jsonschema:"results per page"`
	Page      int    `json:"page,omitempty" jsonschema:"page number, starting at 1"`
}

// ListUserReposOutput is the result of list_user_repos.
type ListUserReposOutput struct {
	Username     string        `json:"username"`
	Page         int           `json:"page"`
	PerPage      int           `json:"per_page"`
	Count        int           `json:"count" jsonschema:"number of repositories on this page"`
	HasMore      bool          `json:"has_more" jsonschema:"true if another page exists; request page+1 to get it"`
	Repositories []RepoSummary `json:"repositories"`
}

// ListUserRepos is the logic of list_user_repos.
func ListUserRepos(ctx context.Context, gh *github.Client, in ListUserReposInput) (ListUserReposOutput, error) {
	username := strings.TrimSpace(in.Username)
	if err := validateUsername(username); err != nil {
		return ListUserReposOutput{}, err
	}
	if err := validateChoice("sort", in.Sort, listSortValues); err != nil {
		return ListUserReposOutput{}, err
	}
	if err := validateChoice("direction", in.Direction, listDirectionValues); err != nil {
		return ListUserReposOutput{}, err
	}
	perPage, page, err := resolvePaging(in.PerPage, in.Page, listDefaultPerPage)
	if err != nil {
		return ListUserReposOutput{}, err
	}

	repos, hasMore, err := gh.ListUserRepos(ctx, username, github.ListUserReposOptions{
		Sort:      in.Sort,
		Direction: in.Direction,
		PerPage:   perPage,
		Page:      page,
	})
	if err != nil {
		return ListUserReposOutput{}, githubError(err, fmt.Sprintf("repositories of GitHub user %q", username))
	}

	return ListUserReposOutput{
		Username:     username,
		Page:         page,
		PerPage:      perPage,
		Count:        len(repos),
		HasMore:      hasMore,
		Repositories: toRepoSummaries(repos),
	}, nil
}

// AddListUserReposTool registers list_user_repos.
func AddListUserReposTool(server *mcp.Server, gh *github.Client) {
	tool := &mcp.Tool{
		Name: "list_user_repos",
		Description: "List public repositories owned by a GitHub user or organization, one page at a time. " +
			"Each item has name, description, language, stars and forks. Use has_more and page to get more.",
		Annotations: &mcp.ToolAnnotations{Title: "List a user's repositories", ReadOnlyHint: true},
		InputSchema: inputSchema[ListUserReposInput](func(props map[string]*jsonschema.Schema) {
			props["sort"].Enum = enum(listSortValues)
			props["direction"].Enum = enum(listDirectionValues)
			addPagingRules(props, listDefaultPerPage)
		}),
	}
	mcp.AddTool(server, tool, func(ctx context.Context, req *mcp.CallToolRequest, in ListUserReposInput) (*mcp.CallToolResult, ListUserReposOutput, error) {
		out, err := ListUserRepos(ctx, gh, in)
		return nil, out, err
	})
}
