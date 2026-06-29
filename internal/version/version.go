// Package version holds build-time version metadata injected via goreleaser
// ldflags (see .goreleaser.yaml and the Makefile) and derives values from it,
// such as the HTTP User-Agent sent on every API request for backend telemetry.
package version

import "fmt"

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
	version = v
	commit = c
	date = d
}

// Version returns the build version string (e.g. "1.2.3" or "dev").
func Version() string { return version }

// Commit returns the build commit hash.
func Commit() string { return commit }

// Date returns the build date.
func Date() string { return date }

// UserAgent returns the HTTP User-Agent header value, e.g. "wk/1.2.3".
// The backend telemetry pipeline attributes requests to the CLI and its
// version via this header (mirroring the old workato-platform-cli's
// "workato-platform-cli/<version>" convention).
func UserAgent() string {
	return fmt.Sprintf("wk/%s", version)
}
