package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/workato-devs/wk/internal/commands"
	wkversion "github.com/workato-devs/wk/internal/version"
)

// Set by goreleaser ldflags. These names must stay in sync with the -X
// flags in .goreleaser.yaml and the Makefile (main.version/commit/date).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Record build-time version info so every package (CLI version command,
	// API client User-Agent, MCP client) reads from a single source.
	wkversion.Set(version, commit, date)
	os.Exit(commands.Execute(ctx))
}
