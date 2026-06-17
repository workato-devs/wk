package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/workato-devs/wk/internal/commands"
)

// Set by goreleaser ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	commands.SetVersionInfo(version, commit, date)
	os.Exit(commands.Execute(ctx))
}
