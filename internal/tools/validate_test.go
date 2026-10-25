package tools

import (
	"strings"
	"testing"
	"time"
)

func TestValidateUsername(t *testing.T) {
	valid := []string{"octocat", "a", "torvalds", "my-org", "A1-b2", strings.Repeat("a", 39)}
	invalid := []string{"", "-octocat", "bad/name", "has space", "dot.name", "under_score", strings.Repeat("a", 40)}

	for _, name := range valid {
		if err := validateUsername(name); err != nil {
			t.Errorf("validateUsername(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range invalid {
		if err := validateUsername(name); err == nil {
			t.Errorf("validateUsername(%q) = nil, want an error", name)
		}
	}
}

func TestValidateRepoName(t *testing.T) {
	valid := []string{"go", "Hello-World", "my_repo", "octocat.github.io", ".github"}
	invalid := []string{"", ".", "..", "golang/go", "has space", strings.Repeat("a", 101)}

	for _, name := range valid {
		if err := validateRepoName(name); err != nil {
			t.Errorf("validateRepoName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range invalid {
		if err := validateRepoName(name); err == nil {
			t.Errorf("validateRepoName(%q) = nil, want an error", name)
		}
	}
}

func TestValidateChoice(t *testing.T) {
	allowed := []string{"asc", "desc"}
	if err := validateChoice("order", "", allowed); err != nil {
		t.Errorf("empty value should mean default, got %v", err)
	}
	if err := validateChoice("order", "desc", allowed); err != nil {
		t.Errorf("allowed value rejected: %v", err)
	}
	err := validateChoice("order", "sideways", allowed)
	if err == nil || !strings.Contains(err.Error(), "asc, desc") {
		t.Errorf("error should list the allowed values, got %v", err)
	}
}

func TestResolvePaging(t *testing.T) {
	tests := []struct {
		perPage, page         int
		wantPerPage, wantPage int
		wantErr               bool
	}{
		{0, 0, 30, 1, false}, // zero values mean "use defaults"
		{10, 2, 10, 2, false},
		{100, 1, 100, 1, false},
		{101, 1, 0, 0, true},
		{-1, 1, 0, 0, true},
		{10, -3, 0, 0, true},
	}
	for _, tt := range tests {
		perPage, page, err := resolvePaging(tt.perPage, tt.page, 30)
		if (err != nil) != tt.wantErr {
			t.Errorf("resolvePaging(%d, %d) error = %v, wantErr %v", tt.perPage, tt.page, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && (perPage != tt.wantPerPage || page != tt.wantPage) {
			t.Errorf("resolvePaging(%d, %d) = %d, %d; want %d, %d", tt.perPage, tt.page, perPage, page, tt.wantPerPage, tt.wantPage)
		}
	}
}

func TestFormatTime(t *testing.T) {
	if got := formatTime(time.Time{}); got != "" {
		t.Errorf("zero time = %q, want empty", got)
	}
	if got := formatTime(time.Date(2011, 1, 25, 18, 44, 36, 0, time.UTC)); got != "2011-01-25T18:44:36Z" {
		t.Errorf("got %q", got)
	}
}
