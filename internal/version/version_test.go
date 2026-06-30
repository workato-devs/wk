package version

import "testing"

func TestSetPopulatesAccessors(t *testing.T) {
	Set("1.2.3", "abc1234", "2026-06-30")

	if got, want := Version(), "1.2.3"; got != want {
		t.Errorf("Version() = %q, want %q", got, want)
	}
	if got, want := Commit(), "abc1234"; got != want {
		t.Errorf("Commit() = %q, want %q", got, want)
	}
	if got, want := Date(), "2026-06-30"; got != want {
		t.Errorf("Date() = %q, want %q", got, want)
	}
}

func TestUserAgentFormat(t *testing.T) {
	Set("1.2.3", "c", "d")

	if got, want := UserAgent(), "workato-cli/1.2.3"; got != want {
		t.Errorf("UserAgent() = %q, want %q", got, want)
	}
}

// A leading "v" from a git tag (e.g. via `git describe` in the Makefile) must
// be normalized so the telemetry token is consistent across build paths.
func TestSetStripsLeadingV(t *testing.T) {
	Set("v1.0.6-beta", "c", "d")

	if got, want := Version(), "1.0.6-beta"; got != want {
		t.Errorf("Version() = %q, want %q (leading v should be stripped)", got, want)
	}
	if got, want := UserAgent(), "workato-cli/1.0.6-beta"; got != want {
		t.Errorf("UserAgent() = %q, want %q", got, want)
	}
}

// The dev fallback must not be mangled by the "v" trimming.
func TestSetDevFallbackUnchanged(t *testing.T) {
	Set("dev", "none", "unknown")

	if got, want := UserAgent(), "workato-cli/dev"; got != want {
		t.Errorf("UserAgent() = %q, want %q", got, want)
	}
}
