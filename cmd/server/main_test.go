package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github-mcp-server/internal/github"
)

// connect starts the real server (with a fake GitHub behind it) and an MCP
// client, joined by in-memory transports instead of stdio. This tests the
// whole path an AI client uses: schema validation, handlers and results.
func connect(t *testing.T, githubHandler http.HandlerFunc) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	fake := httptest.NewServer(githubHandler)
	t.Cleanup(fake.Close)
	server := newServer(github.NewClient(github.Options{BaseURL: fake.URL}))

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil) // server first, then client
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// resultText returns the text of the first content item of a tool result.
func resultText(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	if text, ok := res.Content[0].(*mcp.TextContent); ok {
		return text.Text
	}
	return ""
}

func TestListTools(t *testing.T) {
	session := connect(t, func(w http.ResponseWriter, r *http.Request) {})

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if tool.Description == "" {
			t.Errorf("tool %s has no description", tool.Name)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %s should be marked read-only", tool.Name)
		}
	}
	slices.Sort(names)
	want := []string{"get_github_repo", "get_github_user", "list_user_repos", "search_repositories"}
	if !slices.Equal(names, want) {
		t.Errorf("tools = %v, want %v", names, want)
	}
}

func TestCallGetGitHubUser(t *testing.T) {
	session := connect(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"login":"octocat","name":"The Octocat","public_repos":8,"created_at":"2011-01-25T18:44:36Z"}`))
	})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_github_user",
		Arguments: map[string]any{"username": "octocat"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}

	// StructuredContent arrives as generic JSON; round-trip it into a struct.
	data, _ := json.Marshal(res.StructuredContent)
	var out struct {
		Login       string `json:"login"`
		PublicRepos int    `json:"public_repos"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decoding structured content: %v", err)
	}
	if out.Login != "octocat" || out.PublicRepos != 8 {
		t.Errorf("structured content = %s", data)
	}
}

// TestHTTPTransport runs the same server behind newHTTPHandler on a real
// local HTTP server and talks to it with the SDK's streamable HTTP client,
// the way a remote client such as ChatGPT would.
func TestHTTPTransport(t *testing.T) {
	ctx := context.Background()

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"login":"octocat","public_repos":8}`))
	}))
	t.Cleanup(fake.Close)
	mcpServer := httptest.NewServer(newHTTPHandler(newServer(github.NewClient(github.Options{BaseURL: fake.URL}))))
	t.Cleanup(mcpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: mcpServer.URL + mcpPath}, nil)
	if err != nil {
		t.Fatalf("client connect over HTTP: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) != 4 {
		t.Errorf("got %d tools over HTTP, want 4", len(tools.Tools))
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_github_user",
		Arguments: map[string]any{"username": "octocat"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError || !strings.Contains(resultText(res), `"login":"octocat"`) {
		t.Errorf("result = %q (IsError %v)", resultText(res), res.IsError)
	}
}

// TestHTTPRejectsForeignHost documents why tunnels must rewrite the Host
// header: a request reaching a loopback address with a public Host is refused.
func TestHTTPRejectsForeignHost(t *testing.T) {
	mcpServer := httptest.NewServer(newHTTPHandler(newServer(github.NewClient(github.Options{}))))
	t.Cleanup(mcpServer.Close)

	req, _ := http.NewRequest(http.MethodPost, mcpServer.URL+mcpPath, strings.NewReader(`{}`))
	req.Host = "abc123.ngrok-free.app"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestToolErrors(t *testing.T) {
	session := connect(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	})

	tests := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"github 404", "get_github_repo", map[string]any{"owner": "octocat", "repo": "nope"}, "was not found"},
		{"missing argument", "get_github_user", map[string]any{}, "username"},
		{"wrong type", "list_user_repos", map[string]any{"username": "octocat", "per_page": "ten"}, "per_page"},
		{"enum violated", "list_user_repos", map[string]any{"username": "octocat", "sort": "stars"}, "sort"},
		{"maximum violated", "search_repositories", map[string]any{"query": "mcp", "per_page": 500}, "per_page"},
		{"handler validation", "get_github_user", map[string]any{"username": "bad/name"}, "not a valid GitHub username"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tt.tool, Arguments: tt.args})
			if err != nil {
				t.Fatalf("CallTool returned a protocol error, want a tool error: %v", err)
			}
			if !res.IsError {
				t.Fatalf("IsError = false, want true")
			}
			if text := resultText(res); !strings.Contains(text, tt.want) {
				t.Errorf("error text %q does not contain %q", text, tt.want)
			}
		})
	}
}
