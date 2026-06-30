// Package version holds build-time version metadata injected via goreleaser
// ldflags (see .goreleaser.yaml and the Makefile) and derives values from it,
// such as the HTTP User-Agent sent on every API request for backend telemetry.
package version

import (
	"fmt"
	"strings"
)

// These are populated once at startup by Set, sourced from the values
// goreleaser injects into main via ldflags. They default to dev values so
// `go run`/`go test` builds still produce a sensible User-Agent.
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

// UserAgent returns the HTTP User-Agent header value, e.g. "workato-cli/1.2.3".
// The backend telemetry pipeline attributes requests to the CLI and its
// version via this header, mirroring the old workato-platform-cli's
// "workato-platform-cli/<version>" convention.
//
// The "workato-cli" token is deliberately distinctive. The nginx access-log
// field this lands in is analyzed/tokenized, so a short token like "wk"
// collides with unrelated traffic (it appears as a stray token in other
// user-agents), whereas "workato-cli" matches cleanly as a phrase.
func UserAgent() string {
	return fmt.Sprintf("workato-cli/%s", version)
}
