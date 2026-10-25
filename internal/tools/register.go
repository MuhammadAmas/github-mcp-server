package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github-mcp-server/internal/github"
)

// Register adds every GitHub tool to server. All tools share gh, so they
// share its connection pool, timeout and (optional) token.
//
// To add a new tool: write it in its own file with an AddXxxTool function,
// then call that function here.
func Register(server *mcp.Server, gh *github.Client) {
	AddGetUserTool(server, gh)
	AddGetRepoTool(server, gh)
	AddListUserReposTool(server, gh)
	AddSearchRepositoriesTool(server, gh)
}
