// Package tools defines the MCP tools this server exposes. Each tool turns
// MCP input into a call on the GitHub client and the result into MCP output.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MuhammadAmas/github-mcp-server/internal/github"
)

// GetUserInput is the argument schema for get_github_user.
type GetUserInput struct {
	Username string `json:"username" jsonschema:"GitHub login of a user or organization, for example octocat"`
}

// GetUserOutput is what get_github_user returns as structured content.
// Fields marked omitempty are left out when GitHub has no value for them.
type GetUserOutput struct {
	Login       string `json:"login"`
	Type        string `json:"type" jsonschema:"User or Organization"`
	Name        string `json:"name,omitempty"`
	Company     string `json:"company,omitempty"`
	Blog        string `json:"blog,omitempty"`
	Location    string `json:"location,omitempty"`
	Email       string `json:"email,omitempty" jsonschema:"public email, if the user chose to show one"`
	Bio         string `json:"bio,omitempty"`
	PublicRepos int    `json:"public_repos"`
	PublicGists int    `json:"public_gists"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
	ProfileURL  string `json:"profile_url"`
	AvatarURL   string `json:"avatar_url"`
	CreatedAt   string `json:"created_at" jsonschema:"account creation time, RFC 3339"`
}

// GetUser is the tool's logic, kept free of MCP types so it is easy to test.
func GetUser(ctx context.Context, gh *github.Client, in GetUserInput) (GetUserOutput, error) {
	username := strings.TrimSpace(in.Username)
	if err := validateUsername(username); err != nil {
		return GetUserOutput{}, err
	}

	user, err := gh.GetUser(ctx, username)
	if err != nil {
		return GetUserOutput{}, githubError(err, fmt.Sprintf("GitHub user %q", username))
	}

	return GetUserOutput{
		Login:       user.Login,
		Type:        user.Type,
		Name:        user.Name,
		Company:     user.Company,
		Blog:        user.Blog,
		Location:    user.Location,
		Email:       user.Email,
		Bio:         user.Bio,
		PublicRepos: user.PublicRepos,
		PublicGists: user.PublicGists,
		Followers:   user.Followers,
		Following:   user.Following,
		ProfileURL:  user.HTMLURL,
		AvatarURL:   user.AvatarURL,
		CreatedAt:   formatTime(user.CreatedAt),
	}, nil
}

// AddGetUserTool registers get_github_user on server, using gh for API calls.
func AddGetUserTool(server *mcp.Server, gh *github.Client) {
	tool := &mcp.Tool{
		Name: "get_github_user",
		Description: "Get the public profile of a GitHub user or organization: name, bio, " +
			"company, location, follower counts, number of public repositories and profile URL.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get GitHub user",
			ReadOnlyHint: true, // only reads data, never changes anything on GitHub
		},
	}

	// This anonymous function is a closure: it "captures" gh from the
	// surrounding function, so every call can use the same GitHub client.
	mcp.AddTool(server, tool, func(ctx context.Context, req *mcp.CallToolRequest, in GetUserInput) (*mcp.CallToolResult, GetUserOutput, error) {
		out, err := GetUser(ctx, gh, in)
		return nil, out, err
	})
}
