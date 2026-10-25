// Command ghuser is a small developer tool for trying the GitHub client
// from the terminal, without MCP:
//
//	go run ./cmd/ghuser octocat
//
// It is not an MCP server, so printing to stdout is fine here.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MuhammadAmas/github-mcp-server/internal/github"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: ghuser <github-username>")
		os.Exit(2)
	}
	username := os.Args[1]

	client := github.NewClient(github.Options{
		Token:   os.Getenv("GITHUB_TOKEN"),   // optional; empty means unauthenticated
		BaseURL: os.Getenv("GITHUB_API_URL"), // optional; empty means api.github.com
	})

	// Give the whole operation at most 15 seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	user, err := client.GetUser(ctx, username)
	if err != nil {
		switch {
		case github.IsNotFound(err):
			fmt.Fprintf(os.Stderr, "user %q does not exist on GitHub\n", username)
		case github.IsRateLimited(err):
			fmt.Fprintln(os.Stderr, "rate limited:", err)
		default:
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(1)
	}

	fmt.Println("Login:       ", user.Login)
	fmt.Println("Name:        ", user.Name)
	fmt.Println("Type:        ", user.Type)
	fmt.Println("Company:     ", user.Company)
	fmt.Println("Location:    ", user.Location)
	fmt.Println("Public repos:", user.PublicRepos)
	fmt.Println("Followers:   ", user.Followers)
	fmt.Println("Profile:     ", user.HTMLURL)
	fmt.Println("Joined:      ", user.CreatedAt.Format("2006-01-02"))
}
