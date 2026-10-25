package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github-mcp-server/internal/github"
)

// fakeGitHub starts a fake GitHub API that answers with handler, and returns
// a client pointed at it plus a counter of how many requests it received.
func fakeGitHub(t *testing.T, handler http.HandlerFunc) (*github.Client, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32 // atomic: the server runs handlers on other goroutines
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return github.NewClient(github.Options{BaseURL: srv.URL}), &calls
}

// respond returns a handler that always writes status and body.
func respond(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}
}

// ---------- get_github_user ----------

func TestGetUser_MapsFields(t *testing.T) {
	gh, _ := fakeGitHub(t, respond(200, `{"login":"octocat","type":"User","name":"The Octocat",
		"bio":null,"public_repos":8,"followers":5,"html_url":"https://github.com/octocat",
		"created_at":"2011-01-25T18:44:36Z"}`))

	out, err := GetUser(context.Background(), gh, GetUserInput{Username: "octocat"})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if out.Login != "octocat" || out.Name != "The Octocat" || out.PublicRepos != 8 || out.Followers != 5 {
		t.Errorf("unexpected output: %+v", out)
	}
	if out.ProfileURL != "https://github.com/octocat" || out.CreatedAt != "2011-01-25T18:44:36Z" {
		t.Errorf("ProfileURL/CreatedAt wrong: %+v", out)
	}

	// omitempty: a null bio must not appear in the JSON sent to the AI.
	data, _ := json.Marshal(out)
	if strings.Contains(string(data), `"bio"`) {
		t.Errorf("empty bio should be omitted: %s", data)
	}
}

func TestGetUser_InvalidInputMakesNoRequest(t *testing.T) {
	gh, calls := fakeGitHub(t, respond(200, `{}`))
	for _, name := range []string{"", "   ", "bad/name", "-dash"} {
		if _, err := GetUser(context.Background(), gh, GetUserInput{Username: name}); err == nil {
			t.Errorf("GetUser(%q) succeeded, want a validation error", name)
		}
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("made %d GitHub requests for invalid input, want 0", n)
	}
}

func TestGetUser_ErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"not found", respond(404, `{"message":"Not Found"}`), `GitHub user "ghost-user" was not found`},
		{"rate limited", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(403)
		}, "Wait and try again later"},
		{"server error", respond(502, `{"message":"Bad Gateway"}`), "Bad Gateway"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gh, _ := fakeGitHub(t, tt.handler)
			_, err := GetUser(context.Background(), gh, GetUserInput{Username: "ghost-user"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

// ---------- get_github_repo ----------

func TestGetRepo_NoLicense(t *testing.T) {
	gh, _ := fakeGitHub(t, respond(200, `{"full_name":"a/b","name":"b","owner":{"login":"a"},
		"license":null,"pushed_at":null,"stargazers_count":3,"subscribers_count":2}`))

	out, err := GetRepo(context.Background(), gh, GetRepoInput{Owner: "a", Repo: "b"})
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if out.License != "" || out.PushedAt != "" {
		t.Errorf("License/PushedAt should be empty for null values: %+v", out)
	}
	if out.Stars != 3 || out.Watchers != 2 || out.Owner != "a" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestGetRepo_UnclassifiedLicense(t *testing.T) {
	gh, _ := fakeGitHub(t, respond(200, `{"full_name":"a/b","license":{"spdx_id":"NOASSERTION","name":"Other"}}`))
	out, err := GetRepo(context.Background(), gh, GetRepoInput{Owner: "a", Repo: "b"})
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if out.License != "Other" {
		t.Errorf("License = %q, want Other", out.License)
	}
}

func TestGetRepo_OwnerInRepoField(t *testing.T) {
	gh, calls := fakeGitHub(t, respond(200, `{}`))
	_, err := GetRepo(context.Background(), gh, GetRepoInput{Owner: "octocat", Repo: "golang/go"})
	if err == nil || !strings.Contains(err.Error(), "owner argument") {
		t.Errorf("error = %v, want a hint about the owner argument", err)
	}
	if calls.Load() != 0 {
		t.Error("should not call GitHub for invalid input")
	}
}

// ---------- list_user_repos ----------

func TestListUserRepos_DefaultsAndHasMore(t *testing.T) {
	gh, _ := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("per_page") != "30" || q.Get("page") != "1" {
			t.Errorf("defaults not applied: %v", q)
		}
		w.Header().Set("Link", `<https://api.github.com/x?page=2>; rel="next"`)
		w.Write([]byte(`[{"full_name":"octocat/a","stargazers_count":7,"license":null}]`))
	})

	out, err := ListUserRepos(context.Background(), gh, ListUserReposInput{Username: "octocat"})
	if err != nil {
		t.Fatalf("ListUserRepos: %v", err)
	}
	if out.Page != 1 || out.PerPage != 30 || out.Count != 1 || !out.HasMore {
		t.Errorf("unexpected paging info: %+v", out)
	}
	if out.Repositories[0].FullName != "octocat/a" || out.Repositories[0].Stars != 7 {
		t.Errorf("unexpected repo: %+v", out.Repositories[0])
	}
}

func TestListUserRepos_EmptyListIsNotNull(t *testing.T) {
	gh, _ := fakeGitHub(t, respond(200, `[]`))
	out, err := ListUserRepos(context.Background(), gh, ListUserReposInput{Username: "empty-user"})
	if err != nil {
		t.Fatalf("ListUserRepos: %v", err)
	}
	data, _ := json.Marshal(out)
	if !strings.Contains(string(data), `"repositories":[]`) {
		t.Errorf("want an empty JSON list, got %s", data)
	}
}

func TestListUserRepos_RejectsBadOptions(t *testing.T) {
	gh, calls := fakeGitHub(t, respond(200, `[]`))
	bad := []ListUserReposInput{
		{Username: "octocat", Sort: "stars"},
		{Username: "octocat", Direction: "up"},
		{Username: "octocat", PerPage: 500},
		{Username: "octocat", Page: -1},
	}
	for _, in := range bad {
		if _, err := ListUserRepos(context.Background(), gh, in); err == nil {
			t.Errorf("ListUserRepos(%+v) succeeded, want an error", in)
		}
	}
	if calls.Load() != 0 {
		t.Error("should not call GitHub for invalid input")
	}
}

// ---------- search_repositories ----------

func TestSearchRepositories(t *testing.T) {
	gh, _ := fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") != "mcp language:go" || q.Get("per_page") != "10" || q.Get("sort") != "stars" {
			t.Errorf("unexpected query: %v", q)
		}
		w.Write([]byte(`{"total_count":2,"incomplete_results":false,
			"items":[{"full_name":"a/mcp","stargazers_count":9},{"full_name":"b/mcp","stargazers_count":4}]}`))
	})

	out, err := SearchRepositories(context.Background(), gh, SearchReposInput{Query: " mcp language:go ", Sort: "stars"})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}
	if out.Query != "mcp language:go" || out.TotalCount != 2 || out.Count != 2 || out.PerPage != 10 {
		t.Errorf("unexpected output: %+v", out)
	}
	if out.Repositories[0].FullName != "a/mcp" {
		t.Errorf("unexpected first result: %+v", out.Repositories[0])
	}
}

func TestSearchRepositories_RejectsBadInput(t *testing.T) {
	gh, calls := fakeGitHub(t, respond(200, `{}`))
	bad := []SearchReposInput{
		{Query: ""},
		{Query: strings.Repeat("x", maxQueryLength+1)},
		{Query: "mcp", Sort: "popularity"},
		{Query: "mcp", Page: 101},              // beyond the first 1000 results with per_page 10
		{Query: "mcp", PerPage: 100, Page: 11}, // also beyond 1000
	}
	for _, in := range bad {
		if _, err := SearchRepositories(context.Background(), gh, in); err == nil {
			t.Errorf("SearchRepositories(query len %d, %+v) succeeded, want an error", len(in.Query), in.Page)
		}
	}
	if calls.Load() != 0 {
		t.Error("should not call GitHub for invalid input")
	}
}

func TestSearchRepositories_LastAllowedPage(t *testing.T) {
	gh, _ := fakeGitHub(t, respond(200, `{"total_count":5000,"items":[]}`))
	if _, err := SearchRepositories(context.Background(), gh, SearchReposInput{Query: "mcp", PerPage: 10, Page: 100}); err != nil {
		t.Errorf("page 100 with per_page 10 is within the first 1000 results: %v", err)
	}
}

func TestSearchRepositories_ValidationFailed(t *testing.T) {
	gh, _ := fakeGitHub(t, respond(422, `{"message":"Validation Failed","errors":[{"message":"The listed users cannot be searched"}]}`))
	_, err := SearchRepositories(context.Background(), gh, SearchReposInput{Query: "user:ghost-zz9"})
	if err == nil || !strings.Contains(err.Error(), "The listed users cannot be searched") {
		t.Errorf("error = %v, want GitHub's reason included", err)
	}
}
