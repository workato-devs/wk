package commands

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/workato-devs/wk/internal/mcp"
)

// --bearer must only ever send the profile token to an https Workato host.
func TestIsWorkatoHTTPS(t *testing.T) {
	tests := map[string]bool{
		"https://app.workato.com/airo_mcp":              true,
		"https://app.trial.workato.com/airo_mcp":        true,
		"https://2761.apim.mcp.trial.workato.com":       true,
		"https://app.workatoapp.cn/airo_mcp":            true,
		"http://app.workato.com/airo_mcp":               false, // not https
		"https://example.com/airo_mcp":                  false,
		"https://workato.com.evil.example/airo_mcp":     false, // suffix trick
		"https://evilworkato.com/airo_mcp":              false, // no dot boundary
		"https://app.workato.com@evil.example/airo_mcp": false, // userinfo trick
		"not a url": false,
	}
	for raw, want := range tests {
		if got := isWorkatoHTTPS(raw); got != want {
			t.Errorf("isWorkatoHTTPS(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestMCPConnectError(t *testing.T) {
	challenge := &mcp.HTTPError{StatusCode: 401, Status: "401 Unauthorized", WWWAuthenticate: `Bearer resource_metadata="x"`}
	airo := "https://app.trial.workato.com/airo_mcp"

	if err := mcpConnectError(challenge, airo, false); !strings.Contains(err.Error(), "rerun with --bearer") {
		t.Errorf("no --bearer: %v, want it to suggest --bearer", err)
	}
	if err := mcpConnectError(challenge, "https://example.com/mcp", false); strings.Contains(err.Error(), "--bearer") {
		t.Errorf("non-Workato host: %v, must not suggest sending the token there", err)
	}
	resetGlobalFlags(t)
	flagProfile = "dev"
	if err := mcpConnectError(challenge, airo, true); !strings.Contains(err.Error(), `rejected profile "dev"'s token`) {
		t.Errorf("with --bearer: %v, want it to name the rejected profile", err)
	}
	other := fmt.Errorf("wrapped: %w", errors.New("boom"))
	if err := mcpConnectError(other, airo, false); err != other {
		t.Errorf("non-HTTP error changed: %v", err)
	}
}
