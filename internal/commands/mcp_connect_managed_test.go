package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedactURLToken(t *testing.T) {
	got := redactURLToken("https://1.apim.mcp.trial.workato.com/ns/coll?wkt_token=abc123secretvalue")
	if strings.Contains(got, "abc123secretvalue") || !strings.Contains(got, "wkt_token=REDACTED") {
		t.Errorf("redactURLToken = %s", got)
	}
	if got := redactURLToken("https://2761.apim.mcp.trial.workato.com"); got != "https://2761.apim.mcp.trial.workato.com" {
		t.Errorf("URL without a token changed: %s", got)
	}
}

func TestMarshalConfig_NoHTMLEscaping(t *testing.T) {
	cfg, err := marshalConfig(map[string]string{"url": "<placeholder>?a=1&b=2"})
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg) != `{"url":"<placeholder>?a=1&b=2"}` {
		t.Errorf("config = %s, want it copy-pasteable", cfg)
	}
}

func TestManagedPlan(t *testing.T) {
	claude, codex, cursor := findConnectClient("claude-code"), findConnectClient("codex"), findConnectClient("cursor")
	opts := func(scope string) connectOptions { return connectOptions{Scope: scope, Name: "workato-srv"} }

	t.Run("token URL can't go in a project config", func(t *testing.T) {
		d := &managedDetails{Method: methodToken, URL: "https://x?wkt_token=REDACTED", URLRedacted: true}
		for _, c := range []*connectClient{claude, cursor} {
			if _, err := managedPlan(c, d, opts("project")); err == nil {
				t.Errorf("%s: want an error at project scope", c.Name)
			}
		}
	})
	t.Run("redacted entry says how to get the full one", func(t *testing.T) {
		d := &managedDetails{Method: methodToken, URL: "https://x?wkt_token=REDACTED", URLRedacted: true}
		c, err := managedPlan(claude, d, opts("local"))
		if err != nil || !strings.Contains(c.Note, "--allow-url-token") {
			t.Errorf("note = %q, err = %v", c.Note, err)
		}
	})
	t.Run("workato_idp is URL only, then sign in", func(t *testing.T) {
		d := &managedDetails{Method: methodIDP, URL: "https://2761.apim.mcp.trial.workato.com", OAuth: &oauthDetails{Available: true, SignInFirst: "sign in"}}
		c, _ := managedPlan(claude, d, opts("project"))
		if string(c.Config) != `{"type":"http","url":"https://2761.apim.mcp.trial.workato.com"}` {
			t.Errorf("claude config = %s", c.Config)
		}
		c, _ = managedPlan(codex, d, opts("user"))
		if c.Then[0] != "Then run: codex mcp login workato-srv" {
			t.Errorf("codex next step = %q", c.Then[0])
		}
		c, _ = managedPlan(cursor, d, opts("user"))
		if !strings.Contains(string(c.Config), "mcp-remote") {
			t.Errorf("cursor config = %s, want the mcp-remote form", c.Config)
		}
	})
	t.Run("hashed_token gets a placeholder", func(t *testing.T) {
		d := &managedDetails{Method: methodHashedToken, URL: urlPlaceholder, Unavailable: "token not readable"}
		c, _ := managedPlan(claude, d, opts("local"))
		if !strings.Contains(string(c.Config), urlPlaceholder) || !strings.Contains(c.Note, "Replace") {
			t.Errorf("config = %s note = %q", c.Config, c.Note)
		}
	})
}

// The registration check follows the server's protected-resource metadata to
// its authorization server, which for workato_idp servers is Workato
// Identity, not the app.* server AIRO uses.
func TestResourceRegistration(t *testing.T) {
	as := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/servers/abc/.well-known/oauth-authorization-server" {
			w.Write([]byte(`{"registration_endpoint":"https://id/auth/servers/abc/clients"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer as.Close()
	rs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/oauth-protected-resource" {
			w.Write([]byte(`{"authorization_servers":["` + as.URL + `/auth/servers/abc"]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer rs.Close()

	discovery, ok, err := resourceRegistration(context.Background(), rs.URL)
	if err != nil || !ok {
		t.Fatalf("got ok=%v err=%v; want registration supported", ok, err)
	}
	if discovery != as.URL+"/auth/servers/abc/.well-known/oauth-authorization-server" {
		t.Errorf("discovery = %s", discovery)
	}

	// A server with no resource metadata (hashed_token servers) is an error,
	// not a silent "no".
	if _, _, err := resourceRegistration(context.Background(), as.URL); err == nil {
		t.Error("want an error when there's no protected-resource metadata")
	}
}

func TestConnectTargetAndFlagErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"unknown target", []string{"mcp", "connect", "nope"}, "unknown connect target"},
		{"--auth on a managed server", []string{"mcp", "connect", "mcps-AbC123-x", "--auth", "token"}, "--auth doesn't apply to managed MCP servers"},
		{"--allow-url-token on airo", []string{"mcp", "connect", "airo", "--allow-url-token"}, "--allow-url-token applies to managed MCP servers"},
		{"--config-scope without --client", []string{"mcp", "connect", "airo", "--config-scope", "user"}, "--config-scope applies to a client's config"},
		{"unsupported config scope", []string{"mcp", "connect", "airo", "--client", "codex", "--config-scope", "project"}, "Codex doesn't support --config-scope project"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags(t)
			setupIsolatedHome(t)
			root := NewRootCmd()
			root.AddCommand(newMCPCmd())
			root.SetArgs(tt.args)
			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
