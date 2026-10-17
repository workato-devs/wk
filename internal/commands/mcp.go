package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server management and testing",
	}
	cmd.AddCommand(newMCPTestCmd())
	cmd.AddCommand(newMCPToolsCmd())
	cmd.AddCommand(newMCPServersCmd())
	cmd.AddCommand(newMCPUserGroupsCmd())
	cmd.AddCommand(newMCPConnectCmd())
	cmd.AddCommand(newMCPAuthHeaderCmd())
	return cmd
}

func newMCPTestCmd() *cobra.Command {
	var flagBearer bool
	cmd := &cobra.Command{
		Use:   "test <url>",
		Short: "Test MCP server connectivity",
		Long: `Run the MCP initialize handshake against a server and report what it says.

Managed servers carry their credential in the URL. First-party Workato
endpoints such as AIRO MCP (https://app.<region>.workato.com/airo_mcp) need a
bearer token instead: pass --bearer to send the active profile's token. It is
sent only to https Workato hosts.`,
		Example: `  wk mcp test https://app.workato.com/mcp/abcdef
  wk mcp test https://app.trial.workato.com/airo_mcp --bearer
  wk mcp test https://app.workato.com/mcp/abcdef --json`,
		Args: requireArgs(1, "server URL is required, e.g.: wk mcp test <url>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}

			client, err := newMCPClientForCmd(cmd, args[0], flagBearer)
			if err != nil {
				return err
			}
			info, err := client.Initialize(cmd.Context())
			if err != nil {
				return fmt.Errorf("MCP server test failed: %w", mcpConnectError(err, args[0], flagBearer))
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, info)
			}

			fmt.Fprintf(os.Stdout, "Server:   %s\n", info.Name)
			fmt.Fprintf(os.Stdout, "Version:  %s\n", info.Version)
			fmt.Fprintf(os.Stdout, "Protocol: %s\n", info.ProtocolVersion)
			if len(info.Capabilities) > 0 {
				capJSON, _ := json.Marshal(info.Capabilities)
				fmt.Fprintf(os.Stdout, "Capabilities: %s\n", string(capJSON))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagBearer, "bearer", false, "Send the active profile's API token as a bearer header (https Workato hosts only)")
	return cmd
}

func newMCPToolsCmd() *cobra.Command {
	var flagBearer bool
	cmd := &cobra.Command{
		Use:   "tools <url>",
		Short: "List tools exposed by an MCP server",
		Long: `List the tools an MCP server exposes.

For first-party Workato endpoints such as AIRO MCP, pass --bearer to send the
active profile's token (https Workato hosts only).`,
		Example: `  wk mcp tools https://app.workato.com/mcp/abcdef
  wk mcp tools https://app.trial.workato.com/airo_mcp --bearer
  wk mcp tools https://app.workato.com/mcp/abcdef --json`,
		Args: requireArgs(1, "server URL is required, e.g.: wk mcp tools <url>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}

			client, err := newMCPClientForCmd(cmd, args[0], flagBearer)
			if err != nil {
				return err
			}
			if _, err := client.Initialize(cmd.Context()); err != nil {
				return fmt.Errorf("MCP initialize failed: %w", mcpConnectError(err, args[0], flagBearer))
			}

			tools, err := client.ListTools(cmd.Context())
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, tools)
			}

			headers := []string{"NAME", "DESCRIPTION"}
			var rows [][]string
			for _, t := range tools {
				desc := t.Description
				if len(desc) > 80 {
					desc = desc[:77] + "..."
				}
				rows = append(rows, []string{t.Name, desc})
			}

			fmt.Fprintf(os.Stderr, "%d tools available\n\n", len(tools))
			return rctx.Formatter.FormatList(os.Stdout, tools, headers, rows)
		},
	}
	cmd.Flags().BoolVar(&flagBearer, "bearer", false, "Send the active profile's API token as a bearer header (https Workato hosts only)")
	return cmd
}

// newMCPClientForCmd builds an MCP client for rawURL. With bearer, it sends
// the active profile's token, but only to an https Workato host, so a
// mistyped or hostile URL never receives the credential.
func newMCPClientForCmd(cmd *cobra.Command, rawURL string, bearer bool) (*mcp.Client, error) {
	client := mcp.NewClient(rawURL)
	if !bearer {
		return client, nil
	}
	if !isWorkatoHTTPS(rawURL) {
		return nil, fmt.Errorf("--bearer sends your profile's API token, so it's only allowed for https Workato hosts (*.workato.com, *.workatoapp.cn); %q isn't one", rawURL)
	}
	name, _, err := resolveActiveProfileName()
	if err != nil {
		return nil, err
	}
	_, cred, err := resolveProfileAndCred(cmd.Context(), name)
	if err != nil {
		return nil, err
	}
	return client.WithBearer(cred.Token), nil
}

// isWorkatoHTTPS reports whether rawURL is https on a Workato domain.
func isWorkatoHTTPS(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, domain := range []string{"workato.com", "workatoapp.cn"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// mcpConnectError adds what to do next to a 401 from an MCP server.
func mcpConnectError(err error, rawURL string, bearer bool) error {
	var httpErr *mcp.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusUnauthorized {
		return err
	}
	switch {
	case bearer:
		name, _, _ := resolveActiveProfileName()
		return fmt.Errorf("%w. The server rejected profile %q's token: check the profile is for this server's workspace and region, and that its API client has the permission this endpoint needs (for AIRO MCP: \"AIRO MCP Server\")", err, name)
	case strings.HasPrefix(strings.ToLower(httpErr.WWWAuthenticate), "bearer") && isWorkatoHTTPS(rawURL):
		return fmt.Errorf("%w. This endpoint wants a bearer token; rerun with --bearer to send the active profile's API token", err)
	default:
		return err
	}
}
