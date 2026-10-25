package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MuhammadAmas/github-mcp-server/internal/github"
)

var (
	searchSortValues  = []string{"stars", "forks", "help-wanted-issues", "updated"}
	searchOrderValues = []string{"desc", "asc"}
)

const (
	searchDefaultPerPage = 10
	maxQueryLength       = 256  // GitHub rejects longer search queries
	maxSearchResults     = 1000 // GitHub only returns the first 1000 matches
)

// SearchReposInput is the argument schema for search_repositories.
type SearchReposInput struct {
	Query   string `json:"query" jsonschema:"GitHub search query. Plain words match name, description and README; qualifiers narrow it, e.g. language:go, user:octocat, topic:mcp, stars:>1000, pushed:>2024-01-01"`
	Sort    string `json:"sort,omitempty" jsonschema:"sort field; omit for best match"`
	Order   string `json:"order,omitempty" jsonschema:"sort order when sort is set; default desc"`
	PerPage int    `json:"per_page,omitempty" jsonschema:"results per page"`
	Page    int    `json:"page,omitempty" jsonschema:"page number, starting at 1"`
}

// SearchReposOutput is the result of search_repositories.
type SearchReposOutput struct {
	Query             string        `json:"query"`
	TotalCount        int           `json:"total_count" jsonschema:"total matches on GitHub (only the first 1000 can be paged through)"`
	IncompleteResults bool          `json:"incomplete_results" jsonschema:"true if GitHub timed out and the results may be partial"`
	Page              int           `json:"page"`
	PerPage           int           `json:"per_page"`
	Count             int           `json:"count" jsonschema:"number of repositories on this page"`
	Repositories      []RepoSummary `json:"repositories"`
}

// SearchRepositories is the logic of search_repositories.
func SearchRepositories(ctx context.Context, gh *github.Client, in SearchReposInput) (SearchReposOutput, error) {
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return SearchReposOutput{}, fmt.Errorf("query is required")
	}
	if len(query) > maxQueryLength {
		return SearchReposOutput{}, fmt.Errorf("query is %d characters; GitHub allows at most %d", len(query), maxQueryLength)
	}
	if err := validateChoice("sort", in.Sort, searchSortValues); err != nil {
		return SearchReposOutput{}, err
	}
	if err := validateChoice("order", in.Order, searchOrderValues); err != nil {
		return SearchReposOutput{}, err
	}
	perPage, page, err := resolvePaging(in.PerPage, in.Page, searchDefaultPerPage)
	if err != nil {
		return SearchReposOutput{}, err
	}
	if (page-1)*perPage >= maxSearchResults {
		return SearchReposOutput{}, fmt.Errorf("GitHub search only returns the first %d results; with per_page=%d the last page is %d", maxSearchResults, perPage, maxSearchResults/perPage)
	}

	result, err := gh.SearchRepositories(ctx, query, github.SearchReposOptions{
		Sort:    in.Sort,
		Order:   in.Order,
		PerPage: perPage,
		Page:    page,
	})
	if err != nil {
		return SearchReposOutput{}, githubError(err, fmt.Sprintf("search results for %q", query))
	}

	return SearchReposOutput{
		Query:             query,
		TotalCount:        result.TotalCount,
		IncompleteResults: result.IncompleteResults,
		Page:              page,
		PerPage:           perPage,
		Count:             len(result.Items),
		Repositories:      toRepoSummaries(result.Items),
	}, nil
}

// AddSearchRepositoriesTool registers search_repositories.
func AddSearchRepositoriesTool(server *mcp.Server, gh *github.Client) {
	tool := &mcp.Tool{
		Name: "search_repositories",
		Description: "Search public GitHub repositories using GitHub's search syntax. " +
			"Returns matching repositories with description, language, stars and forks. " +
			"GitHub allows only 10 searches per minute without a token, so prefer one precise query over many broad ones.",
		Annotations: &mcp.ToolAnnotations{Title: "Search GitHub repositories", ReadOnlyHint: true},
		InputSchema: inputSchema[SearchReposInput](func(props map[string]*jsonschema.Schema) {
			props["query"].MinLength = jsonschema.Ptr(1)
			props["query"].MaxLength = jsonschema.Ptr(maxQueryLength)
			props["sort"].Enum = enum(searchSortValues)
			props["order"].Enum = enum(searchOrderValues)
			addPagingRules(props, searchDefaultPerPage)
		}),
	}
	mcp.AddTool(server, tool, func(ctx context.Context, req *mcp.CallToolRequest, in SearchReposInput) (*mcp.CallToolResult, SearchReposOutput, error) {
		out, err := SearchRepositories(ctx, gh, in)
		return nil, out, err
	})
}
