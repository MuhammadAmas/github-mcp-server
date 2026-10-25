package tools

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// usernamePattern matches GitHub logins: 1-39 letters, digits or hyphens,
// not starting with a hyphen.
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

// repoNamePattern matches repository names: 1-100 letters, digits, '.', '-' or '_'.
var repoNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

// maxPerPage is the largest page size GitHub accepts.
const maxPerPage = 100

// validateUsername checks a username before we spend an API call on it.
// The same rules apply to organization names, so we use it for repo owners too.
func validateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("%q is not a valid GitHub username: use 1-39 letters, digits or hyphens, not starting with a hyphen", username)
	}
	return nil
}

// validateRepoName checks a repository name (without the owner).
func validateRepoName(name string) error {
	if name == "" {
		return fmt.Errorf("repo is required")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("repo %q must be just the repository name; put the owner in the owner argument", name)
	}
	if name == "." || name == ".." || !repoNamePattern.MatchString(name) {
		return fmt.Errorf("%q is not a valid repository name: use 1-100 letters, digits, '.', '-' or '_'", name)
	}
	return nil
}

// validateChoice checks that value is empty (meaning "use the default")
// or one of allowed.
func validateChoice(argName, value string, allowed []string) error {
	if value == "" || slices.Contains(allowed, value) {
		return nil
	}
	return fmt.Errorf("%s must be one of %s, got %q", argName, strings.Join(allowed, ", "), value)
}

// resolvePaging validates per_page and page and fills in defaults for zero values.
func resolvePaging(perPage, page, defaultPerPage int) (int, int, error) {
	if perPage == 0 {
		perPage = defaultPerPage
	}
	if page == 0 {
		page = 1
	}
	if perPage < 1 || perPage > maxPerPage {
		return 0, 0, fmt.Errorf("per_page must be between 1 and %d, got %d", maxPerPage, perPage)
	}
	if page < 1 {
		return 0, 0, fmt.Errorf("page must be 1 or more, got %d", page)
	}
	return perPage, page, nil
}

// formatTime formats t as RFC 3339, or returns "" for the zero time
// (GitHub sends null for e.g. pushed_at on an empty repository).
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
