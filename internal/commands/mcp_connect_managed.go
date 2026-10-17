package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/config"
)

// Managed MCP servers (`wk mcp connect <mcps-handle>`). How a client
// authenticates is fixed by the server's authentication method, not chosen
// with --auth (ADR-008 Decision 8):
//
//   - workato_idp: OAuth through Workato Identity. The config holds only the
//     URL; the client registers itself and the user signs in.
//   - token: the URL itself carries the credential (?wkt_token=). wk writes it
//     into a client config only with --allow-url-token, never at project
//     scope, and otherwise prints it redacted.
//   - hashed_token: the token can't be read through the API (and token-renew
//     doesn't return it), so wk can't configure a client; it prints the entry
//     with a placeholder for the URL copied from the Workato UI.

const (
	methodIDP         = "workato_idp"
	methodToken       = "token"
	methodHashedToken = "hashed_token"

	urlPlaceholder = "<server URL from the Workato UI>"
)

// managedDetails is the client-neutral description of a managed server
// connection.
type managedDetails struct {
	Status      string         `json:"status"` // "printed" or "added"
	Target      string         `json:"target"` // "managed-server"
	Handle      string         `json:"handle"`
	Name        string         `json:"name"`
	Method      string         `json:"authentication_method"`
	URL         string         `json:"url"`
	URLRedacted bool           `json:"url_redacted,omitempty"`
	Transport   string         `json:"transport"`
	Profile     string         `json:"profile"`
	OAuth       *oauthDetails  `json:"oauth,omitempty"`
	URLToken    *urlTokenInfo  `json:"url_token,omitempty"`
	Unavailable string         `json:"unavailable,omitempty"` // why wk can't configure a client
	Client      *clientDetails `json:"client,omitempty"`
}

type urlTokenInfo struct {
	ActsAs        string `json:"acts_as"`
	Setup         string `json:"setup"`
	RevealCommand string `json:"reveal_command"`
}

var managedHandle = regexp.MustCompile(`^mcps-[A-Za-z0-9_-]+$`)

func runManagedConnect(cmd *cobra.Command, rctx *RunContext, handle string, client *connectClient, o connectOptions, nameSet, printOnly, allowURLToken bool) error {
	ctx := cmd.Context()
	apiClient, profile, err := resolveAPIClient(cmd)
	if err != nil {
		return err
	}
	srv, err := apiClient.MCPServers().Get(ctx, handle)
	if err != nil {
		return fmt.Errorf("getting MCP server %s: %w", handle, err)
	}
	method, err := managedServerMethod(ctx, apiClient, srv)
	if err != nil {
		return err
	}

	d := &managedDetails{
		Status: "printed", Target: "managed-server", Handle: srv.ID, Name: srv.Name, Method: method,
		URL: srv.MCPURL, Transport: "streamable-http",
	}
	if profile != nil {
		d.Profile = profile.Name
	}
	if !nameSet {
		o.Name = "workato-mcp-server"
		if slug := slugify(srv.Name); slug != "" {
			o.Name = "workato-" + slug
		}
	}

	switch method {
	case methodIDP:
		discovery, supported, derr := resourceRegistration(ctx, srv.MCPURL)
		d.OAuth = &oauthDetails{
			ActsAs:       "the Workato Identity user who signs in (access follows the server's user groups)",
			Setup:        "Configure the server URL only. The client registers itself with the server's authorization server (Workato Identity) and opens a browser to sign in.",
			SignInFirst:  "Sign in with your Workato Identity account when the browser opens; you need to be in one of the server's user groups.",
			DiscoveryURL: discovery,
		}
		switch {
		case derr != nil:
			d.OAuth.Reason = fmt.Sprintf("couldn't check the authorization server (%v)", derr)
		case !supported:
			d.OAuth.Reason = "its authorization server doesn't support client registration, so an MCP client can't sign in"
		default:
			d.OAuth.Available = true
		}
	case methodToken:
		d.URLToken = &urlTokenInfo{
			ActsAs:        "anyone holding the URL: the server's token is in it (?wkt_token=)",
			Setup:         "Configure the full URL; there's no separate credential. Keep it out of any shared or committed config.",
			RevealCommand: "wk mcp servers get " + srv.ID + " --json",
		}
		if !allowURLToken {
			d.URL, d.URLRedacted = redactURLToken(srv.MCPURL), true
		}
	case methodHashedToken:
		d.URL = urlPlaceholder
		d.Unavailable = "this server uses hashed_token authentication: its token can't be read through the API, and token-renew doesn't return the new one. Copy the server URL from the server's page in the Workato UI and add it to your client yourself."
	default:
		d.Unavailable = fmt.Sprintf("this server's authentication method (%q) isn't one wk knows how to configure; configure the client from the Workato UI", method)
	}

	if client != nil {
		c, err := managedPlan(client, d, o)
		if err != nil {
			return err
		}
		d.Client = c
		if client.Install != nil && !printOnly {
			switch {
			case d.Unavailable != "":
				return fmt.Errorf("can't install %s: %s Rerun with --print to see the entry", handle, d.Unavailable)
			case method == methodToken && !allowURLToken:
				return fmt.Errorf("%s's URL contains its token, so installing writes that credential into %s's config. Rerun with --allow-url-token to accept that (local or user config scope only), or with --print to see the entry with the token redacted", handle, client.Label)
			case method == methodIDP && !d.OAuth.Available:
				return fmt.Errorf("OAuth isn't available for %s: %s", handle, d.OAuth.Reason)
			}
			if err := client.Install(c, o); err != nil {
				return err
			}
			d.Status = "added"
		}
	}

	if flagJSON {
		return rctx.Formatter.Format(os.Stdout, d)
	}
	if d.Status == "added" {
		printManagedInstalled(d)
		return nil
	}
	printManagedDetails(d)
	return nil
}

// managedServerMethod reads the server's authentication method, which GET
// /mcp/mcp_servers/:id doesn't return but the list endpoint does.
func managedServerMethod(ctx context.Context, c api.Client, srv *api.MCPManagedServer) (string, error) {
	folder := srv.FolderID
	for page := 1; page <= 50; page++ {
		opts := &api.MCPServerListOptions{Page: page, PerPage: 100}
		if folder != 0 {
			opts.FolderID = &folder
		}
		servers, err := c.MCPServers().List(ctx, opts)
		if err != nil {
			return "", fmt.Errorf("reading the authentication method of %s: %w", srv.ID, err)
		}
		for _, s := range servers {
			if s.ID == srv.ID {
				return s.AuthenticationMethod, nil
			}
		}
		if len(servers) < 100 {
			break
		}
	}
	return "", fmt.Errorf("couldn't find %s when listing MCP servers, so its authentication method is unknown", srv.ID)
}

// resourceRegistration follows a server's protected-resource metadata
// (RFC 9728) to its authorization server and reports whether that server
// supports dynamic client registration. Returns the authorization-server
// metadata URL it checked.
func resourceRegistration(ctx context.Context, mcpURL string) (string, bool, error) {
	u, err := url.Parse(mcpURL)
	if err != nil || u.Host == "" {
		return "", false, fmt.Errorf("server URL %q is not valid", mcpURL)
	}
	resourceMeta := u.Scheme + "://" + u.Host + "/.well-known/oauth-protected-resource" + strings.TrimSuffix(u.Path, "/")

	var meta struct {
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := getJSON(ctx, resourceMeta, &meta); err != nil {
		return resourceMeta, false, err
	}
	if len(meta.AuthorizationServers) == 0 {
		return resourceMeta, false, fmt.Errorf("%s lists no authorization server", resourceMeta)
	}
	asMeta := strings.TrimSuffix(meta.AuthorizationServers[0], "/") + "/.well-known/oauth-authorization-server"
	supported, err := registrationSupported(ctx, asMeta)
	return asMeta, supported, err
}

func getJSON(ctx context.Context, rawURL string, into any) error {
	timeout := config.TimeoutDuration(flagTimeout)
	if flagTimeout <= 0 {
		timeout = config.TimeoutDuration(config.DefaultTimeout)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

// managedPlan builds a client's entry for a managed server: a URL-only entry
// for every method, since the URL carries whatever authentication there is.
func managedPlan(client *connectClient, d *managedDetails, o connectOptions) (*clientDetails, error) {
	if d.Method == methodToken && o.Scope == "project" {
		return nil, fmt.Errorf("this server's URL contains its token, so it can't go in a project config (shared, and usually committed). Use --config-scope local or user")
	}
	oauth := d.Method == methodIDP
	c := &clientDetails{
		Name: client.Name, Label: client.Label, Known: true, Installs: client.Install != nil,
		Scope: o.Scope, Entry: o.Name,
	}
	switch client.Name {
	case "claude-code":
		cfg, _ := marshalConfig(map[string]any{"type": "http", "url": d.URL})
		c.Config = cfg
		c.Command = fmt.Sprintf("claude mcp add-json --scope %s %s '<config>'", o.Scope, o.Name)
		if oauth {
			c.Then = []string{fmt.Sprintf("In Claude Code, run /mcp, select %s, and choose Authenticate. %s", o.Name, d.OAuth.SignInFirst)}
		}
	case "codex":
		c.Args = []string{"mcp", "add", o.Name, "--url", d.URL}
		c.Command = "codex " + shellJoin(c.Args)
		c.File = "~/.codex/config.toml"
		if oauth {
			c.Then = []string{"Then run: codex mcp login " + o.Name, d.OAuth.SignInFirst}
		}
	case "cursor":
		c.File = "~/.cursor/mcp.json"
		if o.Scope == "project" {
			c.File = ".cursor/mcp.json"
		}
		entry := map[string]any{"url": d.URL}
		if oauth {
			entry = map[string]any{"command": "npx", "args": []string{"-y", "mcp-remote", d.URL}}
		}
		cfg, _ := marshalConfig(map[string]any{"mcpServers": map[string]any{o.Name: entry}})
		c.Config = cfg
		c.Then = []string{fmt.Sprintf("Add %q under mcpServers in %s (create the file if needed; keep any servers already there).", o.Name, c.File)}
		c.Note = "wk doesn't install into Cursor yet, and hasn't verified this entry in the Cursor app."
	}
	switch {
	case d.Unavailable != "":
		c.Note = strings.TrimSpace("Replace " + urlPlaceholder + " with the server URL copied from the Workato UI, then add the entry. " + c.Note)
	case d.URLRedacted:
		c.Note = strings.TrimSpace("The token is redacted; rerun with --allow-url-token for the full entry. " + c.Note)
	case oauth && !d.OAuth.Available:
		c.Note = strings.TrimSpace("OAuth isn't available (" + d.OAuth.Reason + "). " + c.Note)
	}
	return c, nil
}

// marshalConfig renders a client config without escaping <, > and &, so
// placeholders and URLs stay copy-pasteable (json.Marshal escapes them).
func marshalConfig(v any) (json.RawMessage, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(strings.TrimSpace(b.String())), nil
}

// redactURLToken replaces a URL's wkt_token value so it can be shown safely.
func redactURLToken(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<redacted>"
	}
	q := u.Query()
	if q.Has("wkt_token") {
		q.Set("wkt_token", "REDACTED")
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func printManagedDetails(d *managedDetails) {
	w := os.Stdout
	fmt.Fprintf(w, "MCP server %q (%s), authentication: %s\n", d.Name, d.Handle, d.Method)
	fmt.Fprintf(w, "  URL:        %s (streamable HTTP)\n", d.URL)
	if d.Unavailable != "" {
		fmt.Fprintf(w, "\nCan't configure a client automatically: %s\n", d.Unavailable)
	}
	if o := d.OAuth; o != nil {
		fmt.Fprintf(w, "\nOAuth — acts as %s\n", o.ActsAs)
		if o.Available {
			fmt.Fprintln(w, "  Available:  yes")
		} else {
			fmt.Fprintf(w, "  Available:  no — %s\n", o.Reason)
		}
		fmt.Fprintf(w, "  Setup:      %s\n  Sign in:    %s\n  Discovery:  %s\n", o.Setup, o.SignInFirst, o.DiscoveryURL)
	}
	if t := d.URLToken; t != nil {
		fmt.Fprintf(w, "\nToken in URL — acts as %s\n  Setup:      %s\n  Full URL:   %s\n", t.ActsAs, t.Setup, t.RevealCommand)
	}
	if c := d.Client; c != nil {
		fmt.Fprintf(w, "\n%s — %s config scope, entry %q\n", c.Label, c.Scope, c.Entry)
		if c.File != "" {
			fmt.Fprintf(w, "  File:       %s\n", c.File)
		}
		if c.Command != "" {
			fmt.Fprintf(w, "  Command:    %s\n", c.Command)
		}
		if len(c.Config) > 0 {
			fmt.Fprintf(w, "  Config:     %s\n", c.Config)
		}
		for i, step := range c.Then {
			fmt.Fprintf(w, "  %d.          %s\n", i+1, step)
		}
		if c.Note != "" {
			fmt.Fprintf(w, "  Note:       %s\n", c.Note)
		}
	}
}

func printManagedInstalled(d *managedDetails) {
	c := d.Client
	fmt.Fprintf(os.Stderr, "Added %q to %s (%s config scope): MCP server %q (%s)\n", c.Entry, c.Label, c.Scope, d.Name, d.Method)
	if d.Method == methodToken {
		fmt.Fprintf(os.Stderr, "The entry contains the server's token. Anyone who can read %s's config can use this server.\n", c.Label)
	}
	for _, step := range c.Then {
		fmt.Fprintln(os.Stderr, step)
	}
}
