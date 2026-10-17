package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
)

// Genie client commands beyond list (ADR-009 Decision 1). A client is a
// Headless API credential: it serves at most one genie, and a genie can have
// several. The API's create and regenerate responses carry the only copy of
// an api_key client's key, so those commands print it.

func addGenieClientWriteCmds(cmd *cobra.Command) {
	cmd.AddCommand(newClientGetCmd(), newClientCreateCmd(), newClientUpdateCmd(), newClientDeleteCmd(),
		newClientRegenerateCmd(), newClientAttachCmd(), newClientDetachCmd())
}

var clientTypes = []string{"api_key", "oauth"}

func newClientGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <client-id>",
		Short:   "Get a genie client by ID",
		Long:    "Get one client. For OAuth clients this includes the OAuth client ID and redirect URLs, which list leaves out.",
		Example: `  wk agentic clients get gincl-Abr3xKBY-QaFbd9-AB --json`,
		Args:    requireArgs(1, "client ID is required, e.g.: wk agentic clients get <client-id> (IDs look like gincl-…; see wk agentic clients list)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			c, err := client.GenieClients().Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printClient(rctx, c, "")
		},
	}
}

func newClientCreateCmd() *cobra.Command {
	var name, typ, redirectURL string
	var ipAllowList []string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a genie client",
		Long: `Create a Headless API client, then attach it to a genie with
wk agentic clients attach <genie-id> <client-id>.

For an api_key client, this is the only time the API key is shown (in the
output and under --json as api_key). Store it now; if it's lost, replace it
with wk agentic clients regenerate.`,
		Example: `  wk agentic clients create --name "Concierge web" --json
  wk agentic clients create --name "Concierge SPA" --type oauth --redirect-url https://app.example.com/callback
  wk agentic clients create --name "Concierge backend" --ip-allow-list 203.0.113.0/24`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") {
				return fmt.Errorf("--name is required")
			}
			if !slices.Contains(clientTypes, typ) {
				return fmt.Errorf("--type must be one of %s, got %q", strings.Join(clientTypes, ", "), typ)
			}
			if typ == "oauth" && redirectURL == "" {
				return fmt.Errorf("--redirect-url is required with --type oauth")
			}
			if typ != "oauth" && redirectURL != "" {
				return fmt.Errorf("--redirect-url applies only to --type oauth")
			}
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			c, err := client.GenieClients().Create(cmd.Context(), api.GenieClientCreate{
				ClientName:  name,
				Auth:        api.GenieClientAuth{Type: typ, OAuthRedirectURL: redirectURL},
				IPAllowList: ipAllowList,
			})
			if err != nil {
				return err
			}
			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, c)
			}
			if err := printClient(rctx, &c.GenieClient, fmt.Sprintf("Created genie client %s", c.ClientID)); err != nil {
				return err
			}
			if c.APIKey != "" {
				printAPIKey(c.APIKey)
			} else if c.OAuthClientID != "" {
				fmt.Fprintf(os.Stdout, "OAuth client ID: %s\n", c.OAuthClientID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Client name")
	cmd.Flags().StringVar(&typ, "type", "api_key", "api_key or oauth")
	cmd.Flags().StringVar(&redirectURL, "redirect-url", "", "OAuth redirect URL (required with --type oauth)")
	cmd.Flags().StringSliceVar(&ipAllowList, "ip-allow-list", nil, "IPs or CIDR ranges allowed to use the client (comma-separated or repeated)")
	return cmd
}

func printAPIKey(key string) {
	fmt.Fprintln(os.Stderr, "This is the only time the API key is shown. Store it now.")
	fmt.Fprintf(os.Stdout, "API key: %s\n", key)
}

func newClientUpdateCmd() *cobra.Command {
	var name string
	var ipAllowList, redirectURLs []string
	var clearIPs bool
	cmd := &cobra.Command{
		Use:   "update <client-id>",
		Short: "Rename a genie client or change its IP allow list or redirect URLs",
		Long: `Change only what you pass, then print the client as the API now reports it.
--ip-allow-list and --redirect-url replace the whole list. --redirect-url
applies only to OAuth clients. A client's type can't be changed.`,
		Example: `  wk agentic clients update gincl-Abr3xKBY-QaFbd9-AB --name "Concierge web (prod)"
  wk agentic clients update gincl-Abr3xKBY-QaFbd9-AB --ip-allow-list 203.0.113.0/24,198.51.100.7
  wk agentic clients update gincl-Abr3xKBY-QaFbd9-AB --clear-ip-allow-list`,
		Args: requireArgs(1, "client ID is required, e.g.: wk agentic clients update <client-id> --name …"),
		RunE: func(cmd *cobra.Command, args []string) error {
			set := cmd.Flags().Changed
			var in api.GenieClientUpdate
			if set("name") {
				in.ClientName = &name
			}
			if set("ip-allow-list") && clearIPs {
				return fmt.Errorf("use --ip-allow-list or --clear-ip-allow-list, not both")
			}
			if set("ip-allow-list") {
				in.IPAllowList = &ipAllowList
			}
			if clearIPs {
				in.IPAllowList = &[]string{}
			}
			if set("redirect-url") {
				in.OAuthRedirectURLs = &redirectURLs
			}
			if in == (api.GenieClientUpdate{}) {
				return fmt.Errorf("nothing to update; pass --name, --ip-allow-list, --clear-ip-allow-list or --redirect-url")
			}
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			id, ctx := args[0], cmd.Context()
			if in.OAuthRedirectURLs != nil {
				// The API answers 200 and ignores redirect URLs on an api_key client.
				c, err := client.GenieClients().Get(ctx, id)
				if err != nil {
					return err
				}
				if c.Type != "oauth" {
					return fmt.Errorf("--redirect-url applies only to oauth clients; %s is %s", id, c.Type)
				}
			}
			if err := client.GenieClients().Update(ctx, id, in); err != nil {
				return err
			}
			c, err := client.GenieClients().Get(ctx, id)
			if err != nil {
				return fmt.Errorf("updated genie client %s, but re-reading it failed: %w", id, err)
			}
			if err := printClient(rctx, c, fmt.Sprintf("Updated genie client %s", id)); err != nil {
				return err
			}
			return checkClientUpdate(in, c)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringSliceVar(&ipAllowList, "ip-allow-list", nil, "Replace the IP allow list (IPs or CIDR ranges, comma-separated or repeated)")
	cmd.Flags().BoolVar(&clearIPs, "clear-ip-allow-list", false, "Remove every IP restriction")
	cmd.Flags().StringSliceVar(&redirectURLs, "redirect-url", nil, "Replace the OAuth redirect URLs (comma-separated or repeated)")
	return cmd
}

// checkClientUpdate reports a field the API said it changed but didn't.
func checkClientUpdate(in api.GenieClientUpdate, c *api.GenieClient) error {
	var off []string
	if in.ClientName != nil && c.ClientName != *in.ClientName {
		off = append(off, fmt.Sprintf("name is %q, not %q", c.ClientName, *in.ClientName))
	}
	if in.IPAllowList != nil && !slices.Equal(c.IPAllowList, *in.IPAllowList) {
		off = append(off, fmt.Sprintf("IP allow list is %v, not %v", c.IPAllowList, *in.IPAllowList))
	}
	if in.OAuthRedirectURLs != nil && !slices.Equal(c.OAuthRedirectURLs, *in.OAuthRedirectURLs) {
		off = append(off, fmt.Sprintf("redirect URLs are %v, not %v", c.OAuthRedirectURLs, *in.OAuthRedirectURLs))
	}
	if len(off) > 0 {
		return fmt.Errorf("update returned success, but the client's %s", strings.Join(off, "; "))
	}
	return nil
}

type clientDeleteReport struct {
	ClientID string   `json:"client_id"`
	Deleted  bool     `json:"deleted"`
	GenieIDs []string `json:"detached_genie_ids"` // the genie it served, if any
}

func newClientDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <client-id>",
		Short: "Delete a genie client",
		Long: `Delete a client. Its credentials stop working at once.

The API deletes a client even while it's attached to a running genie, cutting
off every app that uses it. So wk refuses when the client is attached and names
the genie; pass --force to delete it anyway. To stop using a client with a
genie but keep it, use wk agentic clients detach instead.`,
		Example: `  wk agentic clients delete gincl-Abr3xKBY-QaFbd9-AB
  wk agentic clients delete gincl-Abr3xKBY-QaFbd9-AB --force --json`,
		Args: requireArgs(1, "client ID is required, e.g.: wk agentic clients delete <client-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			id := args[0]
			c, err := client.GenieClients().Get(cmd.Context(), id)
			if err != nil {
				return err
			}
			genies := c.GenieIDs
			if genies == nil {
				genies = []string{}
			}
			if len(genies) > 0 && !force {
				return clientInUseError(id, genies)
			}
			if err := client.GenieClients().Delete(cmd.Context(), id); err != nil {
				return err
			}
			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, clientDeleteReport{ClientID: id, Deleted: true, GenieIDs: genies})
			}
			fmt.Fprintf(os.Stderr, "Deleted genie client %s\n", id)
			if len(genies) > 0 {
				fmt.Fprintf(os.Stderr, "It was attached to genie %s, which no longer has it\n", strings.Join(genies, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Delete even if the client is attached to a genie")
	return cmd
}

func clientInUseError(id string, genies []string) error {
	return fmt.Errorf("genie client %s is attached to genie %s. Deleting it invalidates its credentials at once, even while the genie is running.\n"+
		"Keep the client and only detach it: wk agentic clients detach %s %s\n"+
		"Or delete it anyway: wk agentic clients delete %s --force",
		id, strings.Join(genies, ", "), genies[0], id, id)
}

func newClientRegenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "regenerate <client-id>",
		Short: "Replace an api_key client's key",
		Long: `Issue a new API key for an api_key client. The old key stops working at once,
so every app using it needs the new one. The new key is shown only this once.
OAuth clients have no key to regenerate.`,
		Example: `  wk agentic clients regenerate gincl-Abr3xKBY-QaFbd9-AB --json`,
		Args:    requireArgs(1, "client ID is required, e.g.: wk agentic clients regenerate <client-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			key, err := client.GenieClients().Regenerate(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, map[string]any{"client_id": args[0], "api_key": key.APIKey, "updated_at": key.UpdatedAt})
			}
			fmt.Fprintf(os.Stderr, "Regenerated the key for genie client %s; the old key no longer works.\n", args[0])
			printAPIKey(key.APIKey)
			return nil
		},
	}
}

// Attach outcomes.
const (
	outcomeAttached        = "attached"
	outcomeAlreadyAttached = "already_attached"
	outcomeDetached        = "detached"
)

type clientAttachReport struct {
	GenieID  string `json:"genie_id"`
	ClientID string `json:"client_id"`
	Outcome  string `json:"outcome"`
}

func newClientAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach <genie-id> <client-id>",
		Short: "Attach a client to a genie",
		Long: `Let a client call a genie through the Headless API. A client serves one genie
at most; a genie can have several clients. Attaching a client that's already
attached to this genie succeeds as already_attached.`,
		Example: `  wk agentic clients attach gin-AbzGALWh-9h9Q9W-AB gincl-Abr3xKBY-QaFbd9-AB`,
		Args:    requireArgs(2, "genie ID and client ID are required, e.g.: wk agentic clients attach <genie-id> <client-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			genieID, clientID := args[0], args[1]
			outcome := outcomeAttached
			if _, err := client.GenieClients().Attach(cmd.Context(), genieID, clientID); err != nil {
				if outcome, err = explainAttachConflict(cmd.Context(), client, genieID, clientID, err); err != nil {
					return err
				}
			}
			return printAttachReport(rctx, clientAttachReport{GenieID: genieID, ClientID: clientID, Outcome: outcome})
		},
	}
}

// explainAttachConflict turns the API's 409 into what actually happened. The
// API answers a client already on this genie with "Genie has already been
// taken" and one on another genie with a message that doesn't name it.
func explainAttachConflict(ctx context.Context, client api.Client, genieID, clientID string, attachErr error) (string, error) {
	var apiErr *api.APIError
	if !errors.As(attachErr, &apiErr) || apiErr.StatusCode != 409 {
		return "", attachErr
	}
	c, err := client.GenieClients().Get(ctx, clientID)
	if err != nil {
		return "", attachErr
	}
	switch {
	case slices.Contains(c.GenieIDs, genieID):
		return outcomeAlreadyAttached, nil
	case len(c.GenieIDs) > 0:
		other := c.GenieIDs[0]
		return "", fmt.Errorf("genie client %s is already attached to genie %s, and a client serves one genie at most.\n"+
			"Detach it there first: wk agentic clients detach %s %s\n"+
			"Or give this genie its own client: wk agentic clients create --name …", clientID, other, other, clientID)
	}
	return "", attachErr
}

func newClientDetachCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "detach <genie-id> <client-id>",
		Short:   "Detach a client from a genie, keeping the client",
		Long:    "Stop a client from calling a genie. The client and its credentials remain and can be attached to another genie.",
		Example: `  wk agentic clients detach gin-AbzGALWh-9h9Q9W-AB gincl-Abr3xKBY-QaFbd9-AB`,
		Args:    requireArgs(2, "genie ID and client ID are required, e.g.: wk agentic clients detach <genie-id> <client-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, client, err := agenticContext(cmd)
			if err != nil {
				return err
			}
			if err := client.GenieClients().Detach(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			return printAttachReport(rctx, clientAttachReport{GenieID: args[0], ClientID: args[1], Outcome: outcomeDetached})
		},
	}
}

func printAttachReport(rctx *RunContext, r clientAttachReport) error {
	if flagJSON {
		return rctx.Formatter.Format(os.Stdout, r)
	}
	switch r.Outcome {
	case outcomeAttached:
		fmt.Fprintf(os.Stderr, "Attached genie client %s to genie %s\n", r.ClientID, r.GenieID)
	case outcomeAlreadyAttached:
		fmt.Fprintf(os.Stderr, "Genie client %s was already attached to genie %s\n", r.ClientID, r.GenieID)
	default:
		fmt.Fprintf(os.Stderr, "Detached genie client %s from genie %s\n", r.ClientID, r.GenieID)
	}
	return nil
}

func printClient(rctx *RunContext, c *api.GenieClient, note string) error {
	if flagJSON {
		return rctx.Formatter.Format(os.Stdout, c)
	}
	if note != "" {
		fmt.Fprintln(os.Stderr, note)
	}
	headers := []string{"CLIENT ID", "NAME", "TYPE", "STATUS", "GENIES", "IP ALLOW LIST"}
	row := []string{c.ClientID, c.ClientName, c.Type, c.Status, strings.Join(c.GenieIDs, ","), strings.Join(c.IPAllowList, ",")}
	return rctx.Formatter.FormatList(os.Stdout, c, headers, [][]string{row})
}
