package tools

import (
	"errors"
	"fmt"

	"github.com/MuhammadAmas/github-mcp-server/internal/github"
)

// githubError turns an error from the GitHub client into a message written
// for the AI model: what went wrong, and what it can do about it.
//
// what describes the thing we were fetching, e.g. `GitHub user "octocat"`.
// Returning the error from a tool handler makes the SDK send it back as a
// tool result with isError=true, so the model can read it and react.
func githubError(err error, what string) error {
	switch {
	case github.IsNotFound(err):
		return fmt.Errorf("%s was not found. Check the spelling of the name", what)
	case github.IsRateLimited(err):
		return fmt.Errorf("could not fetch %s: %v. Wait and try again later", what, err)
	}

	var apiErr *github.APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("could not fetch %s: %v", what, err)
	}
	// Network problems, timeouts, cancelled requests, bad JSON.
	return fmt.Errorf("could not reach GitHub to fetch %s: %v", what, err)
}
