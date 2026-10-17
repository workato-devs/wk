package commands

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/workato-devs/wk/internal/auth"
)

func TestAiroHost(t *testing.T) {
	tests := map[string]string{
		"https://www.workato.com":       "app.workato.com", // US: API and AIRO hosts differ
		"https://app.eu.workato.com":    "app.eu.workato.com",
		"https://app.trial.workato.com": "app.trial.workato.com",
	}
	for base, want := range tests {
		if got, err := airoHost(base); err != nil || got != want {
			t.Errorf("airoHost(%q) = %q, %v; want %q", base, got, err, want)
		}
	}
	if _, err := airoHost("not a url"); err == nil {
		t.Error("airoHost(invalid) should error")
	}
}

func TestProfileCommand(t *testing.T) {
	tests := []struct {
		p    *auth.Profile
		want string
	}{
		{&auth.Profile{Name: "dev-sandbox", StoreType: auth.StoreKeychain}, "wk auth token --profile dev-sandbox"},
		{&auth.Profile{Name: "ci", StoreType: auth.StoreFile}, "wk auth token --profile ci --store-type file"},
		{&auth.Profile{Name: "my profile's", StoreType: auth.StoreKeychain}, `wk auth token --profile 'my profile'\''s'`},
	}
	for _, tt := range tests {
		if got := profileCommand("wk auth token", tt.p); got != tt.want {
			t.Errorf("profileCommand(%q) = %s, want %s", tt.p.Name, got, tt.want)
		}
	}
}

// Whether OAuth works is decided from what the authorization server
// advertises, not a region list.
func TestRegistrationSupported(t *testing.T) {
	serve := func(status int, body string) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(body))
		}))
		t.Cleanup(srv.Close)
		return srv.URL
	}
	ctx := context.Background()

	if ok, err := registrationSupported(ctx, serve(200, `{"registration_endpoint":"https://x/oauth2/register"}`)); err != nil || !ok {
		t.Errorf("advertised endpoint: got %v, %v; want true", ok, err)
	}
	if ok, err := registrationSupported(ctx, serve(200, `{"issuer":"https://x"}`)); err != nil || ok {
		t.Errorf("no endpoint (trial): got %v, %v; want false", ok, err)
	}
	if _, err := registrationSupported(ctx, serve(503, ``)); err == nil {
		t.Error("HTTP 503 should be an error, not a silent false")
	}
}

func TestClaudeCodeEntry(t *testing.T) {
	keychain := &auth.Profile{Name: "dev", StoreType: auth.StoreKeychain}
	fileStore := &auth.Profile{Name: "ci", StoreType: auth.StoreFile}
	details := func(oauthOK bool) *connectDetails {
		return &connectDetails{
			URL:   "https://app.workato.com/airo_mcp",
			OAuth: oauthDetails{Available: oauthOK, Reason: "no registration"},
			Token: tokenDetails{HeaderJSONCommand: "wk mcp auth-header --profile dev"},
		}
	}
	config := func(t *testing.T, c *clientDetails) map[string]any {
		t.Helper()
		var m map[string]any
		if err := json.Unmarshal(c.Config, &m); err != nil {
			t.Fatalf("config: %v", err)
		}
		return m
	}

	t.Run("oauth is URL only", func(t *testing.T) {
		c, err := claudeCodeEntry(details(true), keychain, connectAuthOAuth, "project", "airo-mcp")
		if err != nil {
			t.Fatal(err)
		}
		if m := config(t, c); len(m) != 2 || m["url"] != "https://app.workato.com/airo_mcp" {
			t.Errorf("config = %v, want only type and url", m)
		}
	})
	t.Run("oauth where unavailable carries a note", func(t *testing.T) {
		c, _ := claudeCodeEntry(details(false), keychain, connectAuthOAuth, "local", "airo-mcp")
		if !strings.Contains(c.Note, "--auth token") {
			t.Errorf("note = %q, want it to name --auth token", c.Note)
		}
	})
	t.Run("token uses a header helper, never the token", func(t *testing.T) {
		c, err := claudeCodeEntry(details(true), keychain, connectAuthToken, "local", "airo-mcp")
		if err != nil {
			t.Fatal(err)
		}
		if got := config(t, c)["headersHelper"]; got != "wk mcp auth-header --profile dev" {
			t.Errorf("headersHelper = %v", got)
		}
	})
	t.Run("token refuses project scope", func(t *testing.T) {
		if _, err := claudeCodeEntry(details(true), keychain, connectAuthToken, "project", "airo-mcp"); err == nil {
			t.Error("want an error for token auth at project scope")
		}
	})
	t.Run("file-store token needs local scope", func(t *testing.T) {
		if _, err := claudeCodeEntry(details(true), fileStore, connectAuthToken, "user", "airo-mcp"); err == nil {
			t.Error("want an error for a profiles.env profile at user scope")
		}
	})
}

func TestInstallClaudeCode(t *testing.T) {
	stub := func(t *testing.T, onPath bool, run func(args []string) ([]byte, error)) *[][]string {
		t.Helper()
		var calls [][]string
		origRun, origLook := claudeRun, claudeLookPath
		t.Cleanup(func() { claudeRun, claudeLookPath = origRun, origLook })
		claudeLookPath = func() error {
			if onPath {
				return nil
			}
			return errors.New("not found")
		}
		claudeRun = func(args ...string) ([]byte, error) {
			calls = append(calls, args)
			return run(args)
		}
		return &calls
	}

	t.Run("adds through claude mcp add-json", func(t *testing.T) {
		calls := stub(t, true, func([]string) ([]byte, error) { return nil, nil })
		if err := installClaudeCode("airo-mcp", "local", `{"type":"http"}`, false); err != nil {
			t.Fatal(err)
		}
		if got := strings.Join((*calls)[0], " "); got != `mcp add-json --scope local airo-mcp {"type":"http"}` {
			t.Errorf("claude args = %s", got)
		}
	})
	t.Run("existing entry names --force", func(t *testing.T) {
		stub(t, true, func([]string) ([]byte, error) {
			return []byte("MCP server airo-mcp already exists in local config"), errors.New("exit 1")
		})
		err := installClaudeCode("airo-mcp", "local", `{}`, false)
		if err == nil || !strings.Contains(err.Error(), "--force") {
			t.Errorf("err = %v, want it to name --force", err)
		}
	})
	t.Run("force removes first and tolerates nothing to remove", func(t *testing.T) {
		calls := stub(t, true, func(args []string) ([]byte, error) {
			if args[1] == "remove" {
				return []byte(`No MCP server named "airo-mcp" in local config`), errors.New("exit 1")
			}
			return nil, nil
		})
		if err := installClaudeCode("airo-mcp", "local", `{}`, true); err != nil {
			t.Fatal(err)
		}
		if len(*calls) != 2 || (*calls)[0][1] != "remove" || (*calls)[1][1] != "add-json" {
			t.Errorf("calls = %v, want remove then add-json", *calls)
		}
	})
	t.Run("missing claude CLI names --print", func(t *testing.T) {
		stub(t, false, func([]string) ([]byte, error) { return nil, nil })
		err := installClaudeCode("airo-mcp", "local", `{}`, false)
		if err == nil || !strings.Contains(err.Error(), "--print") {
			t.Errorf("err = %v, want it to name --print", err)
		}
	})
}

func TestCodexPlan(t *testing.T) {
	d := &connectDetails{
		URL:   "https://app.workato.com/airo_mcp",
		OAuth: oauthDetails{Available: true, SignInFirst: "sign in first"},
		Token: tokenDetails{TokenCommand: "wk auth token --profile dev"},
	}
	p := &auth.Profile{Name: "dev"}

	c, _ := codexPlan(d, p, connectOptions{Auth: connectAuthOAuth, Scope: "user", Name: "airo-mcp"})
	if got := strings.Join(c.Args, " "); got != "mcp add airo-mcp --url https://app.workato.com/airo_mcp" {
		t.Errorf("oauth args = %s", got)
	}
	if c.Then[len(c.Then)-1] != "Then run: codex mcp login airo-mcp" {
		t.Errorf("oauth next step = %q, want codex mcp login", c.Then[len(c.Then)-1])
	}

	c, _ = codexPlan(d, p, connectOptions{Auth: connectAuthToken, Scope: "user", Name: "airo-mcp", TokenEnvVar: "WORKATO_API_TOKEN"})
	if got := strings.Join(c.Args, " "); !strings.HasSuffix(got, "--bearer-token-env-var WORKATO_API_TOKEN") {
		t.Errorf("token args = %s, want the env var, never the token", got)
	}
	if !strings.Contains(c.Then[0], `export WORKATO_API_TOKEN="$(wk auth token --profile dev)"`) {
		t.Errorf("token next step = %q", c.Then[0])
	}
}

func TestCursorPlan(t *testing.T) {
	d := &connectDetails{
		URL:   "https://app.workato.com/airo_mcp",
		OAuth: oauthDetails{Available: true},
		Token: tokenDetails{TokenCommand: "wk auth token --profile dev"},
	}
	p := &auth.Profile{Name: "dev"}
	entry := func(t *testing.T, c *clientDetails) map[string]any {
		t.Helper()
		var cfg struct {
			MCPServers map[string]map[string]any `json:"mcpServers"`
		}
		if err := json.Unmarshal(c.Config, &cfg); err != nil {
			t.Fatalf("config: %v", err)
		}
		return cfg.MCPServers["airo-mcp"]
	}

	c, _ := cursorPlan(d, p, connectOptions{Auth: connectAuthOAuth, Scope: "user", Name: "airo-mcp"})
	if c.Installs || c.File != "~/.cursor/mcp.json" {
		t.Errorf("Installs=%v File=%q, want print-only into ~/.cursor/mcp.json", c.Installs, c.File)
	}
	if e := entry(t, c); e["command"] != "npx" {
		t.Errorf("oauth entry = %v, want Workato's documented mcp-remote form", e)
	}

	c, _ = cursorPlan(d, p, connectOptions{Auth: connectAuthToken, Scope: "project", Name: "airo-mcp", TokenEnvVar: "WORKATO_API_TOKEN"})
	if c.File != ".cursor/mcp.json" {
		t.Errorf("project File = %q", c.File)
	}
	headers, _ := entry(t, c)["headers"].(map[string]any)
	if headers["Authorization"] != "Bearer ${env:WORKATO_API_TOKEN}" {
		t.Errorf("token headers = %v, want an env reference, never the token", headers)
	}
}

func TestInstallCodex(t *testing.T) {
	stub := func(t *testing.T, exists bool) (*[][]string, *[][]string) {
		t.Helper()
		var runs, adds [][]string
		origRun, origAdd, origLook := codexRun, codexAdd, codexLookPath
		t.Cleanup(func() { codexRun, codexAdd, codexLookPath = origRun, origAdd, origLook })
		codexLookPath = func() error { return nil }
		codexRun = func(args ...string) ([]byte, error) {
			runs = append(runs, args)
			if args[1] == "get" && !exists {
				return []byte("Error: No MCP server named 'airo-mcp' found."), errors.New("exit 1")
			}
			return nil, nil
		}
		codexAdd = func(args ...string) (string, bool, error) {
			adds = append(adds, args)
			return "Added global MCP server 'airo-mcp'.\n", false, nil
		}
		return &runs, &adds
	}
	c := &clientDetails{Args: []string{"mcp", "add", "airo-mcp", "--url", "https://x/airo_mcp"}}

	t.Run("new server is added", func(t *testing.T) {
		_, adds := stub(t, false)
		if err := installCodex(c, connectOptions{Name: "airo-mcp"}); err != nil {
			t.Fatal(err)
		}
		if len(*adds) != 1 {
			t.Errorf("adds = %v", *adds)
		}
	})
	// codex mcp add silently overwrites, so wk must refuse without --force.
	t.Run("existing server needs --force", func(t *testing.T) {
		_, adds := stub(t, true)
		err := installCodex(c, connectOptions{Name: "airo-mcp"})
		if err == nil || !strings.Contains(err.Error(), "--force") {
			t.Errorf("err = %v, want it to name --force", err)
		}
		if len(*adds) != 0 {
			t.Error("must not add over an existing server without --force")
		}
	})
	t.Run("--force removes then adds", func(t *testing.T) {
		runs, adds := stub(t, true)
		if err := installCodex(c, connectOptions{Name: "airo-mcp", Force: true}); err != nil {
			t.Fatal(err)
		}
		if (*runs)[len(*runs)-1][1] != "remove" || len(*adds) != 1 {
			t.Errorf("runs = %v adds = %v, want remove then add", *runs, *adds)
		}
	})
}

// codex mcp add writes the entry, then starts an OAuth flow that waits for a
// browser. wk must stop it instead of hanging. Uses a fake codex on PATH that
// behaves the same way, so nothing is registered anywhere.
func TestCodexAdd_StopsOAuthWait(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake codex is a shell script")
	}
	dir := t.TempDir()
	// The child (sleep) starts before any output, as an npm-installed codex's
	// native binary does under its Node wrapper. It holds the output pipe, so
	// killing only the parent would leave wk waiting on it.
	script := "#!/bin/sh\nsleep 30 &\necho \"Added global MCP server 'airo-mcp'.\"\necho 'Detected OAuth support. Starting OAuth flow…'\nwait\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	start := time.Now()
	out, stopped, err := codexAdd("mcp", "add", "airo-mcp", "--url", "https://x/airo_mcp")
	if err != nil {
		t.Fatalf("codexAdd: %v", err)
	}
	if !stopped || !strings.Contains(out, "Added") {
		t.Errorf("stopped=%v out=%q, want the OAuth wait stopped after the entry was added", stopped, out)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("took %s; codexAdd waited on the OAuth flow instead of stopping it", elapsed)
	}
}
