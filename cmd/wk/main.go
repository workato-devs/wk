package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/workato-devs/wk/internal/commands"
	wkversion "github.com/workato-devs/wk/internal/version"
)

// These default to "dev"/"none"/"unknown" and are overwritten at build time
// by the Go linker's -X flags. Two build paths set them:
//   - Makefile:        -X main.version=$(VERSION) ...   (VERSION from `git describe`)
//   - .goreleaser.yaml: -X main.version={{.Version}} ... (release builds in CI)
// The variable names must stay version/commit/date so those -X flags keep
// matching (the linker matches by exact symbol name, e.g. main.version).
// A plain `go run`/`go build` with no -X flags leaves the defaults in place.
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
