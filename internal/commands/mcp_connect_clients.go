package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/workato-devs/wk/internal/auth"
)

// connectOptions are the client-specific choices for `wk mcp connect`.
type connectOptions struct {
	Auth        string
	Scope       string
	Name        string
	TokenEnvVar string
	Force       bool
}

// connectClient is one MCP client wk knows. Every known client gets a
// correct, client-specific config; only clients with an install func are
// installed into, and only through that client's own CLI, never by wk editing
// its config files.
type connectClient struct {
	Name         string
	Label        string
	DefaultScope string
	Scopes       []string
	Plan         func(d *connectDetails, p *auth.Profile, o connectOptions) (*clientDetails, error)
	Install      func(c *clientDetails, o connectOptions) error // nil = print-only
}

var connectClients = []connectClient{
	{
		Name: "claude-code", Label: "Claude Code", DefaultScope: "local", Scopes: []string{"local", "user", "project"},
		Plan: func(d *connectDetails, p *auth.Profile, o connectOptions) (*clientDetails, error) {
			return claudeCodeEntry(d, p, o.Auth, o.Scope, o.Name)
		},
		Install: func(c *clientDetails, o connectOptions) error {
			return installClaudeCode(o.Name, o.Scope, string(c.Config), o.Force)
		},
	},
	{
		Name: "codex", Label: "Codex", DefaultScope: "user", Scopes: []string{"user"},
		Plan: codexPlan, Install: installCodex,
	},
	{
		Name: "cursor", Label: "Cursor", DefaultScope: "user", Scopes: []string{"user", "project"},
		Plan: cursorPlan, // print-only: see cursorPlan
	},
}

func findConnectClient(name string) *connectClient {
	for i := range connectClients {
		if connectClients[i].Name == name {
			return &connectClients[i]
		}
	}
	return nil
}

// connectClientNames lists known clients, marking print-only ones.
func connectClientNames() string {
	var names []string
	for _, c := range connectClients {
		n := c.Name
		if c.Install == nil {
			n += " (print-only)"
		}
		names = append(names, n)
	}
	return strings.Join(names, ", ")
}

// ---- Claude Code ----

// claudeRun runs the claude CLI; a variable so tests can stub it.
var claudeRun = func(args ...string) ([]byte, error) {
	return exec.Command("claude", args...).CombinedOutput()
}

// claudeLookPath reports whether the claude CLI is on PATH; stubbed in tests.
var claudeLookPath = func() error {
	_, err := exec.LookPath("claude")
	return err
}

// claudeCodeEntry builds the Claude Code server entry for the chosen auth.
func claudeCodeEntry(d *connectDetails, p *auth.Profile, authMode, scope, name string) (*clientDetails, error) {
	entry := map[string]any{"type": "http", "url": d.URL}
	c := &clientDetails{
		Name: "claude-code", Label: "Claude Code", Known: true, Installs: true,
		Auth: authMode, Scope: scope, Entry: name,
		Command: fmt.Sprintf("claude mcp add-json --scope %s %s '<config>'", scope, name),
	}
	switch authMode {
	case connectAuthOAuth:
		c.Then = []string{d.OAuth.SignInFirst, fmt.Sprintf("In Claude Code, run /mcp, select %s, and choose Authenticate.", name)}
		if scope == "project" {
			c.Then = append(c.Then, "The entry goes in .mcp.json and holds no credential, so it is safe to commit; each teammate authorizes as themselves.")
		}
		if !d.OAuth.Available {
			c.Note = "OAuth isn't available here (" + d.OAuth.Reason + "); this config won't be able to sign in. Use --auth token."
		}
	case connectAuthToken:
		if scope == "project" {
			return nil, fmt.Errorf("--auth token can't use --config-scope project with claude-code: the config names your personal profile (%q), so it isn't shareable. Use --config-scope local (default) or --config-scope user", p.Name)
		}
		if p.StoreType == auth.StoreFile && scope != "local" {
			return nil, fmt.Errorf("profile %q is stored in profiles.env, which the client can only read from inside this project, so --auth token needs --config-scope local", p.Name)
		}
		// Claude Code's headersHelper runs a command for headers on each
		// connection, so the token is read from wk's store, never written.
		entry["headersHelper"] = d.Token.HeaderJSONCommand
		c.Then = []string{fmt.Sprintf("Claude Code reads the token through `%s` each time it connects.", d.Token.HeaderJSONCommand)}
		c.Note = "Claude Code runs the header helper only after you accept its trust prompt for this folder; open Claude Code here once and accept it, including before headless (claude -p) use."
	}
	cfg, err := marshalConfig(entry)
	if err != nil {
		return nil, err
	}
	c.Config = cfg
	return c, nil
}

// installClaudeCode adds the server through Claude Code's own CLI, so wk never
// edits Claude Code's config files directly.
func installClaudeCode(name, scope, cfg string, force bool) error {
	if err := claudeLookPath(); err != nil {
		return fmt.Errorf("the claude CLI isn't on PATH, so wk can't configure Claude Code. Install Claude Code, or rerun with --print and add the printed config yourself")
	}
	if force {
		// Nothing to replace is fine: claude reports `No MCP server named "x"`.
		if out, err := claudeRun("mcp", "remove", "--scope", scope, name); err != nil &&
			!strings.Contains(string(out), "No MCP server named") {
			return fmt.Errorf("removing existing server %q (%s scope): %s", name, scope, strings.TrimSpace(string(out)))
		}
	}
	out, err := claudeRun("mcp", "add-json", "--scope", scope, name, cfg)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "already exists") {
			return fmt.Errorf("Claude Code already has a server named %q in %s scope. Rerun with --force to replace it, or --name to add another", name, scope)
		}
		return fmt.Errorf("claude mcp add-json failed: %s", msg)
	}
	return nil
}

// ---- Codex ----

// codexRun runs a short codex command; stubbed in tests.
var codexRun = func(args ...string) ([]byte, error) {
	cmd := exec.Command("codex", args...)
	return cmd.CombinedOutput()
}

var codexLookPath = func() error {
	_, err := exec.LookPath("codex")
	return err
}

// codexAdd runs `codex mcp add`. For a URL-only (OAuth) server Codex writes
// the entry and then starts an OAuth flow that waits for a browser, which
// would hang an agent or CI run; the entry is already saved by then, so the
// flow is stopped and the caller is told to run `codex mcp login`. Returns
// the output and whether the OAuth flow was stopped. Stubbed in tests.
var codexAdd = func(args ...string) (string, bool, error) {
	cmd := exec.Command("codex", args...)
	// A real OS pipe, not an io.Writer: with a Writer, os/exec copies output
	// in a goroutine and Wait blocks until every process holding the pipe
	// exits, so a child codex launched would keep wk waiting after the kill.
	pr, pw, err := os.Pipe()
	if err != nil {
		return "", false, err
	}
	defer pr.Close()
	cmd.Stdout, cmd.Stderr = pw, pw
	startInOwnGroup(cmd)
	if err := cmd.Start(); err != nil {
		pw.Close()
		return "", false, err
	}
	pw.Close() // the child holds its own copy
	timer := time.AfterFunc(60*time.Second, func() { killGroup(cmd) })
	defer timer.Stop()

	var (
		mu      sync.Mutex
		out     strings.Builder
		stopped bool
	)
	scanned := make(chan struct{})
	go func() {
		defer close(scanned)
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			line := sc.Text()
			mu.Lock()
			out.WriteString(line + "\n")
			if strings.Contains(line, "Starting OAuth flow") {
				stopped = true
				killGroup(cmd)
			}
			mu.Unlock()
		}
	}()
	waitErr := cmd.Wait()
	pr.Close() // unblock the reader even if a stray child still holds the pipe
	<-scanned

	mu.Lock()
	defer mu.Unlock()
	if stopped {
		return out.String(), true, nil
	}
	return out.String(), false, waitErr
}

func codexPlan(d *connectDetails, p *auth.Profile, o connectOptions) (*clientDetails, error) {
	args := []string{"mcp", "add", o.Name, "--url", d.URL}
	c := &clientDetails{
		Name: "codex", Label: "Codex", Known: true, Installs: true,
		Auth: o.Auth, Scope: o.Scope, Entry: o.Name, File: "~/.codex/config.toml",
	}
	switch o.Auth {
	case connectAuthOAuth:
		c.Then = []string{d.OAuth.SignInFirst, "Then run: codex mcp login " + o.Name}
		if !d.OAuth.Available {
			c.Note = "OAuth isn't available here (" + d.OAuth.Reason + "); Codex won't be able to sign in. Use --auth token."
		}
	case connectAuthToken:
		args = append(args, "--bearer-token-env-var", o.TokenEnvVar)
		c.TokenEnvVar = o.TokenEnvVar
		c.Then = []string{fmt.Sprintf("Set %s in the environment Codex starts from, e.g.: export %s=\"$(%s)\"", o.TokenEnvVar, o.TokenEnvVar, d.Token.TokenCommand)}
		c.Note = fmt.Sprintf("Codex reads the token from $%s when it starts; only the variable's name is written to ~/.codex/config.toml.", o.TokenEnvVar)
	}
	c.Args = args
	c.Command = "codex " + shellJoin(args)
	return c, nil
}

func installCodex(c *clientDetails, o connectOptions) error {
	if err := codexLookPath(); err != nil {
		return fmt.Errorf("the codex CLI isn't on PATH, so wk can't configure Codex. Install Codex, or rerun with --print and run the printed command yourself")
	}
	// codex mcp add silently replaces an existing server, so check first:
	// replacing someone's config needs --force, as with every client.
	if _, err := codexRun("mcp", "get", o.Name); err == nil {
		if !o.Force {
			return fmt.Errorf("Codex already has a server named %q. Rerun with --force to replace it, or --name to add another", o.Name)
		}
		if out, err := codexRun("mcp", "remove", o.Name); err != nil {
			return fmt.Errorf("removing existing Codex server %q: %s", o.Name, strings.TrimSpace(string(out)))
		}
	}
	out, _, err := codexAdd(c.Args...)
	if err != nil || !strings.Contains(out, "Added") {
		return fmt.Errorf("codex mcp add failed: %s", strings.TrimSpace(out))
	}
	return nil
}

// ---- Cursor (print-only) ----

// cursorPlan prints Cursor's mcp.json entry but doesn't install it: on Cursor
// 3.4.20, `cursor --add-mcp` ignored --mcp-workspace and wrote VS Code's
// settings.json ("mcp.servers") rather than ~/.cursor/mcp.json, which is where
// Cursor's agent reads servers from. wk won't edit Cursor's files itself.
func cursorPlan(d *connectDetails, p *auth.Profile, o connectOptions) (*clientDetails, error) {
	c := &clientDetails{
		Name: "cursor", Label: "Cursor", Known: true,
		Auth: o.Auth, Scope: o.Scope, Entry: o.Name, File: "~/.cursor/mcp.json",
	}
	if o.Scope == "project" {
		c.File = ".cursor/mcp.json"
	}
	var entry map[string]any
	switch o.Auth {
	case connectAuthOAuth:
		// Per Workato's AIRO MCP docs, Cursor uses mcp-remote for OAuth.
		entry = map[string]any{"command": "npx", "args": []string{"-y", "mcp-remote", d.URL}}
		c.Then = []string{
			fmt.Sprintf("Add %q under mcpServers in %s (create the file if needed; keep any servers already there).", o.Name, c.File),
			d.OAuth.SignInFirst,
			"Open Cursor's MCP settings and connect it; a browser window opens to authorize. Needs Node.js (npx).",
		}
		if !d.OAuth.Available {
			c.Note = "OAuth isn't available here (" + d.OAuth.Reason + "); this entry won't be able to sign in. Use --auth token. "
		}
	case connectAuthToken:
		entry = map[string]any{"url": d.URL, "headers": map[string]string{"Authorization": "Bearer ${env:" + o.TokenEnvVar + "}"}}
		c.TokenEnvVar = o.TokenEnvVar
		c.Then = []string{
			fmt.Sprintf("Add %q under mcpServers in %s (create the file if needed; keep any servers already there).", o.Name, c.File),
			fmt.Sprintf("Set %s in the environment Cursor starts from, e.g.: export %s=\"$(%s)\", then launch Cursor from that shell (cursor .). Launched from the Dock, it won't see shell exports.", o.TokenEnvVar, o.TokenEnvVar, d.Token.TokenCommand),
		}
	}
	c.Note += "wk doesn't install into Cursor yet, and hasn't verified this entry in the Cursor app; check that the server connects in Cursor's MCP settings."
	cfg, err := marshalConfig(map[string]any{"mcpServers": map[string]any{o.Name: entry}})
	if err != nil {
		return nil, err
	}
	c.Config = cfg
	return c, nil
}

// shellJoin renders args as a copy-pasteable shell command line.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		if shellSafeArg.MatchString(a) {
			quoted[i] = a
		} else {
			quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(quoted, " ")
}
