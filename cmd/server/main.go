// Command server is an MCP server that exposes the GitHub REST API as tools:
// get_github_user, get_github_repo, list_user_repos and search_repositories.
//
// By default it talks to MCP clients over stdio, so stdout is reserved for
// protocol messages and all logging goes to stderr. With -http it serves the
// streamable HTTP transport at /mcp instead, for remote clients like ChatGPT:
//
//	github-mcp-server -http 127.0.0.1:8080
//
// Environment variables (all optional):
//
//	GITHUB_TOKEN    GitHub token; raises the rate limit from 60 to 5000 requests/hour
//	GITHUB_API_URL  API base URL; default https://api.github.com
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MuhammadAmas/github-mcp-server/internal/github"
	"github.com/MuhammadAmas/github-mcp-server/internal/tools"
)

const (
	serverName    = "github-mcp-server"
	serverVersion = "1.1.0"

	// mcpPath is where HTTP mode serves MCP; remote clients use http(s)://host/mcp.
	mcpPath = "/mcp"
)

func main() {
	// stdout carries MCP messages, so all logs must go to stderr.
	log.SetOutput(os.Stderr)
	log.SetPrefix("[" + serverName + "] ")

	if err := run(); err != nil {
		log.Fatalf("server stopped with error: %v", err)
	}
}

// run holds the real work of main. Returning an error (instead of calling
// log.Fatal deep inside) keeps every exit path in one place.
func run() error {
	httpAddr := flag.String("http", "", "serve MCP over HTTP at this address, e.g. 127.0.0.1:8080, instead of stdio")
	flag.Parse()

	gh := github.NewClient(github.Options{
		Token:   os.Getenv("GITHUB_TOKEN"),
		BaseURL: os.Getenv("GITHUB_API_URL"),
	})
	if os.Getenv("GITHUB_TOKEN") != "" {
		log.Println("using GITHUB_TOKEN for GitHub requests") // never log the token itself
	}

	server := newServer(gh)

	// Cancel ctx on Ctrl+C or when the OS asks us to stop, so Run returns cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *httpAddr != "" {
		return serveHTTP(ctx, server, *httpAddr)
	}

	log.Printf("%s %s starting on stdio", serverName, serverVersion)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return err
	}
	log.Println("stopped")
	return nil
}

// serveHTTP serves MCP over streamable HTTP at addr until ctx is cancelled.
func serveHTTP(ctx context.Context, server *mcp.Server, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Handler:           newHTTPHandler(server),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() { errc <- httpServer.Serve(ln) }()
	log.Printf("%s %s listening on http://%s%s", serverName, serverVersion, ln.Addr(), mcpPath)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	// Give in-flight requests a few seconds; open event streams are then cut off.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	log.Println("stopped")
	return nil
}

// newHTTPHandler serves the MCP streamable HTTP transport at mcpPath. Every
// client session shares the same server, so all of them see the same tools.
//
// The SDK rejects requests that reach a loopback address with a non-local
// Host header (DNS rebinding protection), so a tunnel in front of a server
// bound to 127.0.0.1 must rewrite Host, e.g. ngrok's --host-header=rewrite.
func newHTTPHandler(server *mcp.Server) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{SessionTimeout: 30 * time.Minute}, // drop sessions remote clients abandon
	)
	mux := http.NewServeMux()
	mux.Handle(mcpPath, mcpHandler)
	return mux
}

// newServer builds the MCP server with every tool registered.
// It is separate from run so tests can create the same server without stdio.
func newServer(gh *github.Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)
	tools.Register(server, gh)
	return server
}
