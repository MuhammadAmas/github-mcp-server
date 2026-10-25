package github

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// GetUser fetches the public profile of a GitHub user or organization.
// https://docs.github.com/en/rest/users/users#get-a-user
func (c *Client) GetUser(ctx context.Context, username string) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username must not be empty")
	}

	var user User
	// PathEscape stops input like "a/b" or "../x" from changing the URL path.
	// We don't need the response headers here, so we discard them with _.
	if _, err := c.get(ctx, "/users/"+url.PathEscape(username), nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}
