package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/auth"
	"github.com/workato-devs/wk/internal/config"
	"github.com/workato-devs/wk/internal/term"
)

// ADR-008 Decision 8: wk configures an MCP client's connection to AIRO and is
// absent from every request after that. Without --client, wk prints the
// connection details in a client-neutral form that any MCP client can use;
// for a client wk knows, it adds that client's exact config, and installs it
// through the client's own CLI where that works (mcp_connect_clients.go).
// OAuth is the default because it acts as the signed-in user; a profile token
// acts as the workspace owner, so token auth is opt-in only and never a
// fallback.

const (
	connectAuthOAuth = "oauth"
	connectAuthToken = "token"

	identityOAuth = "the user who authorizes in the browser"
	identityToken = "the workspace owner (API tokens are not limited by role or project access)"

	airoOAuthScope = "mcp:workai"

	// defaultTokenEnvVar is the variable name Workato's AIRO MCP setup docs
	// use for clients that read the token from the environment.
	defaultTokenEnvVar = "WORKATO_API_TOKEN"
)

// connectDetails is the client-neutral description of an AIRO MCP
// connection: everything an MCP client needs, whether or not wk knows it.
type connectDetails struct {
	Status      string         `json:"status"` // "printed" or "added"
	Target      string         `json:"target"`
	URL         string         `json:"url"`
	Transport   string         `json:"transport"`
	Profile     string         `json:"profile"`
	Workspace   string         `json:"workspace,omitempty"`
	WorkspaceID int            `json:"workspace_id,omitempty"`
	Environment string         `json:"environment,omitempty"`
	OAuth       oauthDetails   `json:"oauth"`
	Token       tokenDetails   `json:"token"`
	Client      *clientDetails `json:"client,omitempty"`
}

type oauthDetails struct {
	Available           bool   `json:"available"`
	Reason              string `json:"reason,omitempty"` // why not, when unavailable
	ActsAs              string `json:"acts_as"`
	Setup               string `json:"setup"`
	SignInFirst         string `json:"sign_in_first"`
	DiscoveryURL        string `json:"discovery_url"`
	ResourceMetadataURL string `json:"resource_metadata_url"`
	Scope               string `json:"scope"`
}

type tokenDetails struct {
	ActsAs            string `json:"acts_as"`
	Setup             string `json:"setup"`
	Header            string `json:"header"`
	TokenCommand      string `json:"token_command"`
	HeaderJSONCommand string `json:"header_json_command"`
}

// clientDetails is the client-specific part, present only with --client.
type clientDetails struct {
	Name        string          `json:"name"`
	Label       string          `json:"label,omitempty"`
	Known       bool            `json:"known"`    // wk has this client's config format
	Installs    bool            `json:"installs"` // wk can install it (vs print-only)
	Auth        string          `json:"auth,omitempty"`
	Scope       string          `json:"config_scope,omitempty"`
	Entry       string          `json:"entry,omitempty"` // server name in the client config
	File        string          `json:"file,omitempty"`  // where the config lives
	Command     string          `json:"command,omitempty"`
	Args        []string        `json:"-"`
	Config      json.RawMessage `json:"config,omitempty"`
	TokenEnvVar string          `json:"token_env_var,omitempty"`
	Then        []string        `json:"then,omitempty"` // next steps, in order
	Note        string          `json:"note,omitempty"`
}

func newMCPConnectCmd() *cobra.Command {
	var (
		flagClient      string
		flagScope       string
		flagAuth        string
		flagName        string
		flagTokenEnvVar string
		flagPrintOnly   bool
		flagForce       bool
		flagAllowURLTok bool
	)

	cmd := &cobra.Command{
		Use:   "connect <target>",
		Short: "Connect any MCP client (Claude Code, Codex, Cursor, ...) to Workato AIRO",
		Long: `Set up an MCP client's connection to Workato AIRO MCP. wk is not involved in
any request after that.

Without --client, prints the connection details in a client-neutral form that
any MCP client can use: the server URL, how each authentication option works,
and the commands that supply a token. Nothing is installed.

With --client, adds that client's exact config and next steps:
  claude-code   installs via the claude CLI (config scopes: local, user, project)
  codex         installs via the codex CLI (config scope: user)
  cursor        print-only: prints the mcp.json entry (config scopes: user, project)
--config-scope is where the client saves the entry on your machine; it has
nothing to do with Workato projects. "project" scope is meant to be committed.

The target is airo, or a managed MCP server's handle (mcps-…). A managed
server's authentication method decides how clients sign in: workato_idp uses
OAuth through Workato Identity; token puts the credential in the URL, which wk
writes into a client config only with --allow-url-token; hashed_token can't be
configured automatically (its token isn't readable through the API).
Add --print to see a client's config without installing it. For a client wk
doesn't know, --client prints the neutral details.

Authentication:
  --auth oauth   (default) The config holds only the server URL. The client
                 registers itself and you authorize in the browser; it then
                 acts as the user who authorized, with that user's permissions.
                 OAuth connects to the workspace and environment your browser
                 is signed in to, so sign in to the right one first.
  --auth token   The client gets the profile's API token without it being
                 written to the config. It acts as the workspace OWNER, and API
                 tokens are not limited by role or project access. Use for
                 headless runs, or where OAuth isn't available (Developer
                 Sandbox / trial).`,
		Example: `  wk mcp connect airo
  wk mcp connect airo --json
  wk mcp connect airo --client claude-code
  wk mcp connect airo --client claude-code --config-scope project
  wk mcp connect airo --client codex
  wk mcp connect airo --client codex --auth token
  wk mcp connect airo --client cursor
  wk mcp connect mcps-AYcNrsC8-Dd8-AB --client claude-code`,
		Args: requireArgs(1, "target is required, e.g.: wk mcp connect airo"),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			managed := managedHandle.MatchString(target)
			if target != "airo" && !managed {
				return fmt.Errorf("unknown connect target %q; use airo, or a managed MCP server handle (mcps-…, from wk mcp servers list)", target)
			}
			if managed && cmd.Flags().Changed("auth") {
				return fmt.Errorf("--auth doesn't apply to managed MCP servers: each server's authentication method decides how clients sign in (see wk mcp servers list --json, authentication_method)")
			}
			if !managed && flagAllowURLTok {
				return fmt.Errorf("--allow-url-token applies to managed MCP servers whose URL contains a token, not to airo")
			}
			switch flagAuth {
			case connectAuthOAuth, connectAuthToken:
			default:
				return fmt.Errorf("unknown --auth %q; valid: %s, %s", flagAuth, connectAuthOAuth, connectAuthToken)
			}
			client := findConnectClient(flagClient)
			if flagScope != "" && client == nil {
				return fmt.Errorf("--config-scope applies to a client's config; add --client (known: %s)", connectClientNames())
			}
			if client != nil {
				if flagScope == "" {
					flagScope = client.DefaultScope
				}
				if !slices.Contains(client.Scopes, flagScope) {
					return fmt.Errorf("%s doesn't support --config-scope %s; valid: %s", client.Label, flagScope, strings.Join(client.Scopes, ", "))
				}
			}

			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			if managed {
				if client == nil && flagClient != "" {
					return fmt.Errorf("wk doesn't know client %q; omit --client for client-neutral details (known: %s)", flagClient, connectClientNames())
				}
				opts := connectOptions{Scope: flagScope, Name: flagName, TokenEnvVar: flagTokenEnvVar, Force: flagForce}
				return runManagedConnect(cmd, rctx, target, client, opts, cmd.Flags().Changed("name"), flagPrintOnly, flagAllowURLTok)
			}
			profileName, _, err := resolveActiveProfileName()
			if err != nil {
				return err
			}
			profile, _, err := resolveProfileAndCred(cmd.Context(), profileName)
			if err != nil {
				return err
			}

			d, err := buildConnectDetails(cmd.Context(), profile)
			if err != nil {
				return err
			}

			switch {
			case flagClient == "":
				if !flagPrintOnly && !flagJSON && !rctx.Quiet {
					fmt.Fprintf(os.Stderr, "Printed connection details; nothing installed. For a client's exact config, add --client (known: %s).\n",
						connectClientNames())
				}
			case client == nil:
				d.Client = &clientDetails{
					Name: flagClient,
					Note: fmt.Sprintf("wk doesn't know %q, so nothing was installed; configure it from the details above. Known clients: %s.",
						flagClient, connectClientNames()),
				}
			default:
				opts := connectOptions{Auth: flagAuth, Scope: flagScope, Name: flagName, TokenEnvVar: flagTokenEnvVar, Force: flagForce}
				c, err := client.Plan(d, profile, opts)
				if err != nil {
					return err
				}
				d.Client = c
				if client.Install != nil && !flagPrintOnly {
					if flagAuth == connectAuthOAuth && !d.OAuth.Available {
						return fmt.Errorf("OAuth isn't available on %s: %s. Use --auth token instead; the client will then act as the workspace owner", hostOf(d.URL), d.OAuth.Reason)
					}
					if err := client.Install(c, opts); err != nil {
						return err
					}
					d.Status = "added"
				}
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, d)
			}
			if d.Status == "added" {
				printConnectInstalled(d)
				return nil
			}
			printConnectDetails(d)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagClient, "client", "", "MCP client: claude-code, codex, cursor (print-only); omit for client-neutral details")
	cmd.Flags().StringVar(&flagScope, "config-scope", "", "Where the client saves the entry: local, user, or project (project is committed and shared; default depends on the client)")
	cmd.Flags().StringVar(&flagAuth, "auth", connectAuthOAuth, "Authentication for --client: oauth (acts as the signed-in user) or token (acts as the workspace owner)")
	cmd.Flags().StringVar(&flagName, "name", "airo-mcp", "Server name in the client config (managed servers default to workato-<server-name>)")
	cmd.Flags().StringVar(&flagTokenEnvVar, "token-env-var", defaultTokenEnvVar, "With --auth token, the environment variable clients like Codex and Cursor read the token from")
	cmd.Flags().BoolVar(&flagPrintOnly, "print", false, "With --client, print the client's config instead of installing it")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Replace an existing server of the same name")
	cmd.Flags().BoolVar(&flagAllowURLTok, "allow-url-token", false, "For a managed server whose URL contains its token: write that URL into the client config (local or user config scope only)")
	return cmd
}

// buildConnectDetails describes the connection for profile in a form any MCP
// client can use. It never fails on the OAuth check: an unreachable discovery
// document is reported as OAuth being unavailable, with the reason.
func buildConnectDetails(ctx context.Context, p *auth.Profile) (*connectDetails, error) {
	host, err := airoHost(p.BaseURL)
	if err != nil {
		return nil, err
	}
	d := &connectDetails{
		Status:      "printed",
		Target:      "airo",
		URL:         "https://" + host + "/airo_mcp",
		Transport:   "streamable-http",
		Profile:     p.Name,
		Workspace:   p.Workspace,
		WorkspaceID: p.WorkspaceID,
		Environment: p.Environment,
		OAuth: oauthDetails{
			ActsAs:              identityOAuth,
			Setup:               "Configure the server URL only. The client registers itself with Workato's authorization server (dynamic client registration) and opens a browser to authorize.",
			SignInFirst:         authorizeInstruction(p, host),
			DiscoveryURL:        "https://" + host + "/.well-known/oauth-authorization-server",
			ResourceMetadataURL: "https://" + host + "/.well-known/oauth-protected-resource/airo_mcp",
			Scope:               airoOAuthScope,
		},
		Token: tokenDetails{
			ActsAs:            identityToken,
			Setup:             "Send the header on every request. Point your client at the token without copying it into the config: an environment variable set from the token command, or the header command if the client can run a command to get headers. Never paste the token into a committed config.",
			Header:            "Authorization: Bearer <token>",
			TokenCommand:      profileCommand("wk auth token", p),
			HeaderJSONCommand: profileCommand("wk mcp auth-header", p),
		},
	}
	supported, err := registrationSupported(ctx, d.OAuth.DiscoveryURL)
	switch {
	case err != nil:
		d.OAuth.Reason = fmt.Sprintf("couldn't check the authorization server (%v)", err)
	case !supported:
		d.OAuth.Reason = "its authorization server doesn't support client registration (Developer Sandbox / trial workspaces don't), so an MCP client can't sign in"
	default:
		d.OAuth.Available = true
	}
	return d, nil
}

// airoHost returns the AIRO MCP host for a profile's API base URL. It matches
// the API host in every region except US, where the API is www.workato.com
// but AIRO is app.workato.com.
func airoHost(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("profile base URL %q is not a valid URL", baseURL)
	}
	if u.Host == "www.workato.com" {
		return "app.workato.com", nil
	}
	return u.Host, nil
}

func hostOf(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		return u.Host
	}
	return rawURL
}

// registrationSupported reports whether the authorization server advertises a
// dynamic client registration endpoint, which MCP clients need for URL-only
// OAuth. Decided from what the server says, not a hardcoded region list.
func registrationSupported(ctx context.Context, discoveryURL string) (bool, error) {
	timeout := config.TimeoutDuration(flagTimeout)
	if flagTimeout <= 0 {
		timeout = config.TimeoutDuration(config.DefaultTimeout)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d from %s", resp.StatusCode, discoveryURL)
	}
	var meta struct {
		RegistrationEndpoint string `json:"registration_endpoint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return false, fmt.Errorf("decoding %s: %w", discoveryURL, err)
	}
	return meta.RegistrationEndpoint != "", nil
}

// authorizeInstruction tells the user which workspace to sign in to before
// authorizing: OAuth binds to the browser session, not to the wk profile.
func authorizeInstruction(p *auth.Profile, host string) string {
	target := "the workspace this profile targets"
	if p.Workspace != "" {
		target = fmt.Sprintf("workspace %q", p.Workspace)
		if p.WorkspaceID != 0 {
			target = fmt.Sprintf("workspace %q (ID %d)", p.Workspace, p.WorkspaceID)
		}
	}
	if p.Environment != "" {
		target += fmt.Sprintf(", environment %q", p.Environment)
	}
	return fmt.Sprintf("Before authorizing, sign in to %s at https://%s in your browser. OAuth connects to whichever workspace and environment the browser is signed in to.", target, host)
}

var (
	shellSafe    = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	shellSafeArg = regexp.MustCompile(`^[A-Za-z0-9._/:@=+-]+$`)
)

// profileCommand appends the profile (and its store) to a wk command, so the
// command doesn't depend on which profile happens to be active when a client
// runs it.
func profileCommand(base string, p *auth.Profile) string {
	name := p.Name
	if !shellSafe.MatchString(name) {
		name = "'" + strings.ReplaceAll(name, "'", `'\''`) + "'"
	}
	cmd := base + " --profile " + name
	if p.StoreType == auth.StoreFile {
		cmd += " --store-type file"
	}
	return cmd
}

// printConnectDetails writes the client-neutral details (and the client's
// part, if any) for a human reader.
func printConnectDetails(d *connectDetails) {
	w := os.Stdout
	who := fmt.Sprintf("profile %q", d.Profile)
	if d.Workspace != "" {
		who += fmt.Sprintf(" — workspace %q", d.Workspace)
		if d.WorkspaceID != 0 {
			who += fmt.Sprintf(" (ID %d)", d.WorkspaceID)
		}
	}
	if d.Environment != "" {
		who += fmt.Sprintf(", environment %q", d.Environment)
	}
	fmt.Fprintf(w, "AIRO MCP for %s\n", who)
	fmt.Fprintf(w, "  URL:        %s (streamable HTTP)\n\n", d.URL)

	fmt.Fprintf(w, "OAuth — acts as %s\n", d.OAuth.ActsAs)
	if d.OAuth.Available {
		fmt.Fprintln(w, "  Available:  yes")
	} else {
		fmt.Fprintf(w, "  Available:  no — %s\n", d.OAuth.Reason)
	}
	fmt.Fprintf(w, "  Setup:      %s\n", d.OAuth.Setup)
	fmt.Fprintf(w, "  First:      %s\n", d.OAuth.SignInFirst)
	fmt.Fprintf(w, "  Discovery:  %s (scope %s)\n\n", d.OAuth.DiscoveryURL, d.OAuth.Scope)

	fmt.Fprintf(w, "Token — acts as %s\n", d.Token.ActsAs)
	fmt.Fprintf(w, "  Header:     %s\n", d.Token.Header)
	fmt.Fprintf(w, "  Token:      %s\n", d.Token.TokenCommand)
	fmt.Fprintf(w, "  As headers: %s   (prints {\"Authorization\":\"Bearer <token>\"})\n", d.Token.HeaderJSONCommand)
	fmt.Fprintf(w, "  Setup:      %s\n", d.Token.Setup)

	c := d.Client
	if c == nil {
		return
	}
	if !c.Known {
		fmt.Fprintf(w, "\n%s\n  Note:       %s\n", c.Name, c.Note)
		return
	}
	fmt.Fprintf(w, "\n%s — %s auth, %s scope, entry %q\n", c.Label, c.Auth, c.Scope, c.Entry)
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

func printConnectInstalled(d *connectDetails) {
	c := d.Client
	fmt.Fprintf(os.Stderr, "Added %q to %s (%s scope): %s\n", c.Entry, c.Label, c.Scope, d.URL)
	if c.Auth == connectAuthToken {
		fmt.Fprintf(os.Stderr, "%s will act as %s.\n", c.Label, d.Token.ActsAs)
	}
	for _, step := range c.Then {
		fmt.Fprintln(os.Stderr, step)
	}
	if c.Note != "" {
		fmt.Fprintln(os.Stderr, c.Note)
	}
}

func newMCPAuthHeaderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth-header",
		Short: "Print the profile's Authorization header as JSON, for MCP client header helpers",
		Long: `Print {"Authorization":"Bearer <token>"} for the active profile (or --profile).

For MCP clients that can run a command to get request headers, such as Claude
Code's headersHelper, so the token is read from wk's credential store at
connect time instead of being written into the client's config. For clients
that read a token from an environment variable instead, use wk auth token.`,
		Example: `  wk mcp auth-header --profile my-profile`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _, err := resolveActiveProfileName()
			if err != nil {
				return err
			}
			_, cred, err := resolveProfileAndCred(cmd.Context(), name)
			if err != nil {
				return err
			}
			if term.IsTerminal(os.Stdout) {
				fmt.Fprintln(os.Stderr, "Warning: a token is being printed to a terminal; this command is meant to be run by an MCP client")
			}
			return json.NewEncoder(os.Stdout).Encode(map[string]string{"Authorization": "Bearer " + cred.Token})
		},
	}
}
