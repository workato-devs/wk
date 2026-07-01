// Package version holds build-time version metadata injected via goreleaser
// ldflags (see .goreleaser.yaml and the Makefile) and derives values from it,
// such as the HTTP User-Agent sent on every API request for backend telemetry.
package version

import (
	"fmt"
	"strings"
)

// These are populated once at startup by Set. The real values originate from
// the Go linker's -X flags, which are declared in the Makefile and
// .goreleaser.yaml, applied to main.version/commit/date, then handed here via
// Set. They default to dev values so `go run`/`go test` builds (no -X flags)
// still produce a sensible User-Agent.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Set records the build-time version info. Called once from main, before any
// HTTP client is constructed.
func Set(v, c, d string) {
	// Normalize a leading "v". Git tags are "v1.0.6-beta"; `git describe` in
	// the Makefile keeps the "v", while goreleaser's {{.Version}} strips it.
	// Trimming here keeps the version — and the telemetry User-Agent —
	// consistent regardless of which build path produced the binary.
	version = strings.TrimPrefix(v, "v")
	commit = c
	date = d
}

// Version returns the build version string (e.g. "1.2.3" or "dev").
func Version() string { return version }

// Commit returns the build commit hash.
func Commit() string { return commit }

// Date returns the build date.
func Date() string { return date }

// UserAgent returns the HTTP User-Agent header value, e.g. "wk-cli/1.2.3".
// The backend telemetry pipeline attributes requests to the CLI and its
// version via this header, mirroring the old workato-platform-cli's
// "workato-platform-cli/<version>" convention.
//
// The token is "wk-cli", not the bare command name "wk": the nginx access-log
// field this lands in is analyzed/tokenized, so a 2-char token like "wk"
// shows up as a stray token in unrelated user-agents and collides. "wk-cli"
// filters cleanly as a phrase ("wk" then "cli" adjacent), and it matches the
// token the original beta builds already sent, keeping telemetry continuous.
func UserAgent() string {
	return fmt.Sprintf("wk-cli/%s", version)
}
