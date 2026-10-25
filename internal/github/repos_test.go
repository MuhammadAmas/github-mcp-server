package github

import (
	"context"
	"net/http"
	"testing"
)

func TestGetRepo(t *testing.T) {
	client := newTestClient(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/golang/go" {
			t.Errorf("path = %s, want /repos/golang/go", r.URL.Path)
		}
		w.Write([]byte(`{"full_name":"golang/go","owner":{"login":"golang"},"stargazers_count":10,
			"license":{"spdx_id":"BSD-3-Clause"},"topics":["go"]}`))
	})

	repo, err := client.GetRepo(context.Background(), "golang", "go")
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if repo.FullName != "golang/go" || repo.Owner.Login != "golang" || repo.StargazersCount != 10 {
		t.Errorf("unexpected repo: %+v", repo)
	}
	if repo.License == nil || repo.License.SPDXID != "BSD-3-Clause" {
		t.Errorf("License = %+v", repo.License)
	}
}

func TestGetRepo_NullLicense(t *testing.T) {
	client := newTestClient(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"full_name":"a/b","license":null}`))
	})
	repo, err := client.GetRepo(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if repo.License != nil {
		t.Errorf("License = %+v, want nil for JSON null", repo.License)
	}
}

func TestListUserRepos(t *testing.T) {
	client := newTestClient(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/octocat/repos" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		for key, want := range map[string]string{"sort": "updated", "direction": "desc", "per_page": "2", "page": "3"} {
			if got := q.Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		w.Header().Set("Link", `<https://api.github.com/user/1/repos?page=4>; rel="next", <https://api.github.com/user/1/repos?page=9>; rel="last"`)
		w.Write([]byte(`[{"full_name":"octocat/a"},{"full_name":"octocat/b"}]`))
	})

	repos, hasMore, err := client.ListUserRepos(context.Background(), "octocat", ListUserReposOptions{
		Sort: "updated", Direction: "desc", PerPage: 2, Page: 3,
	})
	if err != nil {
		t.Fatalf("ListUserRepos: %v", err)
	}
	if len(repos) != 2 || repos[1].FullName != "octocat/b" {
		t.Errorf("repos = %+v", repos)
	}
	if !hasMore {
		t.Error("hasMore = false, want true when Link has rel=\"next\"")
	}
}

func TestListUserRepos_LastPageAndDefaults(t *testing.T) {
	client := newTestClient(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q, want none when options are empty", r.URL.RawQuery)
		}
		w.Write([]byte(`[]`))
	})
	repos, hasMore, err := client.ListUserRepos(context.Background(), "octocat", ListUserReposOptions{})
	if err != nil {
		t.Fatalf("ListUserRepos: %v", err)
	}
	if len(repos) != 0 || hasMore {
		t.Errorf("repos = %v, hasMore = %v; want empty and false", repos, hasMore)
	}
}

func TestSearchRepositories(t *testing.T) {
	client := newTestClient(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/repositories" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		// Query().Get decodes the URL, so we compare against the original text.
		if got := q.Get("q"); got != "mcp language:go stars:>100" {
			t.Errorf("q = %q", got)
		}
		if q.Get("sort") != "stars" || q.Get("order") != "asc" || q.Get("per_page") != "5" {
			t.Errorf("query = %v", q)
		}
		w.Write([]byte(`{"total_count":42,"incomplete_results":true,"items":[{"full_name":"x/y"}]}`))
	})

	res, err := client.SearchRepositories(context.Background(), "mcp language:go stars:>100", SearchReposOptions{
		Sort: "stars", Order: "asc", PerPage: 5,
	})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}
	if res.TotalCount != 42 || !res.IncompleteResults || len(res.Items) != 1 || res.Items[0].FullName != "x/y" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestSearchRepositories_EmptyQueryMakesNoRequest(t *testing.T) {
	client := newTestClient(t, Options{}, mustNotCall(t))
	if _, err := client.SearchRepositories(context.Background(), " ", SearchReposOptions{}); err == nil {
		t.Fatal("expected an error for an empty query")
	}
}
