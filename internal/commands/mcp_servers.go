package commands

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/output"
)

func newMCPServersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "servers",
		Aliases: []string{"server"},
		Short:   "Manage MCP servers in the workspace",
	}
	cmd.AddCommand(newMCPServersListCmd())
	cmd.AddCommand(newMCPServersGetCmd())
	cmd.AddCommand(newMCPServersCreateCmd())
	cmd.AddCommand(newMCPServersCreateBatchCmd())
	cmd.AddCommand(newMCPServersUpdateCmd())
	cmd.AddCommand(newMCPServersDeleteCmd())
	cmd.AddCommand(newMCPServersTokenRenewCmd())
	cmd.AddCommand(newMCPServersToolsCmd())
	cmd.AddCommand(newMCPServersPoliciesCmd())
	cmd.AddCommand(newMCPServersUserGroupsCmd())
	return cmd
}

func newMCPServersListCmd() *cobra.Command {
	var page, perPage int
	var projectID, folderID int
	var authMethod string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List MCP servers",
		Example: `  wk mcp servers list
  wk mcp servers list --project-id 42
  wk mcp servers list --auth-method token --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			opts := &api.MCPServerListOptions{
				AuthenticationMethod: authMethod,
				Page:                 page,
				PerPage:              perPage,
			}
			if cmd.Flags().Changed("project-id") {
				opts.ProjectID = &projectID
			}
			if cmd.Flags().Changed("folder-id") {
				opts.FolderID = &folderID
			}

			servers, err := client.MCPServers().List(cmd.Context(), opts)
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "AUTH", "TOOLS", "PROJECT", "FOLDER"}
			var rows [][]string
			for _, s := range servers {
				auth := s.AuthenticationMethod
				if auth == "" {
					auth = s.AuthType
				}
				rows = append(rows, []string{
					s.ID,
					s.Name,
					auth,
					strconv.Itoa(s.ToolsCount),
					strconv.Itoa(s.ProjectID),
					strconv.Itoa(s.FolderID),
				})
			}
			meta := output.PageMeta{Page: page, PerPage: perPage, HasNext: perPage > 0 && len(servers) == perPage}
			return rctx.Formatter.FormatPage(os.Stdout, servers, headers, rows, meta)
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "Items per page (max 50)")
	cmd.Flags().IntVar(&projectID, "project-id", 0, "Filter by project ID")
	cmd.Flags().IntVar(&folderID, "folder-id", 0, "Filter by folder ID")
	cmd.Flags().StringVar(&authMethod, "auth-method", "", "Filter by authentication method (token, workato_idp)")
	return cmd
}

func newMCPServersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <handle>",
		Short: "Get MCP server details",
		Example: `  wk mcp servers get mcps-AYcNrsC8-Dd8-AB
  wk mcp servers get mcps-AYcNrsC8-Dd8-AB --json`,
		Args: requireArgs(1, "server handle is required, e.g.: wk mcp servers get <handle>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			srv, err := client.MCPServers().Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, srv)
			}

			fmt.Fprintf(os.Stdout, "ID:          %s\n", srv.ID)
			fmt.Fprintf(os.Stdout, "Name:        %s\n", srv.Name)
			if srv.Description != "" {
				fmt.Fprintf(os.Stdout, "Description: %s\n", srv.Description)
			}
			fmt.Fprintf(os.Stdout, "Auth Type:   %s\n", srv.AuthType)
			fmt.Fprintf(os.Stdout, "Tools:       %d\n", srv.ToolsCount)
			fmt.Fprintf(os.Stdout, "Project ID:  %d\n", srv.ProjectID)
			fmt.Fprintf(os.Stdout, "Folder ID:   %d\n", srv.FolderID)
			if srv.MCPURL != "" {
				fmt.Fprintf(os.Stdout, "MCP URL:     %s\n", srv.MCPURL)
			}
			if srv.APICollection != nil {
				fmt.Fprintf(os.Stdout, "Collection:  %s (ID: %d)\n", srv.APICollection.Name, srv.APICollection.ID)
			}
			if len(srv.Folders) > 0 {
				fmt.Fprintf(os.Stdout, "Folders:\n")
				for _, f := range srv.Folders {
					fmt.Fprintf(os.Stdout, "  - %s (ID: %d)\n", f.Name, f.ID)
				}
			}
			return nil
		},
	}
}

func newMCPServersCreateCmd() *cobra.Command {
	var name, description, collection string
	var folderID int
	var recipes []int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an MCP server",
		Long: `Create an MCP server.

With --collection the server is backed by an API collection. With --recipe
(repeatable, and mutually exclusive with --collection) the server exposes the
given recipes directly as tools (project_assets mode), which is what MCP Apps
require. With neither, an empty project_assets server is created.`,
		Example: `  wk mcp servers create --name "my-server" --folder-id 42 --collection "my-api-collection"
  wk mcp servers create --name "my-server" --folder-id 42 --recipe 277601 --recipe 277602
  wk mcp servers create --name "my-server" --folder-id 42 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if !cmd.Flags().Changed("folder-id") {
				return fmt.Errorf("--folder-id is required")
			}
			if collection != "" && len(recipes) > 0 {
				return fmt.Errorf("--collection and --recipe are mutually exclusive (API collection vs project_assets mode)")
			}

			var assetID *int
			if collection != "" {
				id, err := resolveMCPCollection(cmd.Context(), client, collection)
				if err != nil {
					return err
				}
				assetID = &id
			}

			// Resolve recipe tool descriptors up front so a bad recipe ID
			// fails before the server is created.
			var tools []map[string]any
			if len(recipes) > 0 {
				tools, err = buildRecipeTools(cmd.Context(), client, recipes)
				if err != nil {
					return err
				}
			}

			srv, err := client.MCPServers().Create(cmd.Context(), name, folderID, description, assetID)
			if err != nil {
				return err
			}

			if len(tools) > 0 {
				if err := client.MCPServers().AssignTools(cmd.Context(), srv.ID, tools); err != nil {
					return fmt.Errorf("server %s created, but assigning recipe tools failed: %w", srv.ID, err)
				}
				// Re-fetch so the printed/JSON result reflects the tools.
				if refreshed, gerr := client.MCPServers().Get(cmd.Context(), srv.ID); gerr == nil {
					srv = refreshed
				}
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, srv)
			}

			fmt.Fprintf(os.Stderr, "Created MCP server %q (ID: %s)\n", srv.Name, srv.ID)
			if len(tools) > 0 {
				fmt.Fprintf(os.Stderr, "Assigned %d recipe tool(s)\n", len(tools))
			}
			if srv.MCPURL != "" {
				fmt.Fprintln(os.Stdout, srv.MCPURL)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Server name")
	cmd.Flags().IntVar(&folderID, "folder-id", 0, "Folder ID (required)")
	cmd.Flags().StringVar(&collection, "collection", "", "API collection (name or ID)")
	cmd.Flags().IntSliceVar(&recipes, "recipe", nil, "Recipe ID to expose as a tool (project_assets mode; repeatable)")
	cmd.Flags().StringVar(&description, "description", "", "Server description")
	return cmd
}

// buildRecipeTools turns recipe IDs into MCP tool descriptors. The tools API
// keys each tool on its backing recipe's trigger_application, so we look that
// up per recipe rather than guessing.
func buildRecipeTools(ctx context.Context, client api.Client, recipeIDs []int) ([]map[string]any, error) {
	tools := make([]map[string]any, 0, len(recipeIDs))
	for _, rid := range recipeIDs {
		r, err := client.Recipes().Get(ctx, rid)
		if err != nil {
			return nil, fmt.Errorf("looking up recipe %d: %w", rid, err)
		}
		if r.TriggerApplication == "" {
			return nil, fmt.Errorf("recipe %d has no trigger_application; cannot assign it as an MCP tool", rid)
		}
		tools = append(tools, map[string]any{
			"trigger_application": r.TriggerApplication,
			"id":                  strconv.Itoa(rid),
		})
	}
	return tools, nil
}

// resolveMCPCollection resolves a collection flag value to an integer ID.
// Accepts either a numeric ID or a collection name (resolved via API).
func resolveMCPCollection(ctx context.Context, client api.Client, value string) (int, error) {
	if id, err := strconv.Atoi(value); err == nil {
		return id, nil
	}
	return resolveCollectionName(ctx, client, value)
}

func newMCPServersUpdateCmd() *cobra.Command {
	var name, description, authType string
	var folderID int

	cmd := &cobra.Command{
		Use:   "update <handle>",
		Short: "Update an MCP server",
		Example: `  wk mcp servers update mcps-AYcNrsC8-Dd8-AB --name "new-name"
  wk mcp servers update mcps-AYcNrsC8-Dd8-AB --auth-type workato_idp --json`,
		Args: requireArgs(1, "server handle is required, e.g.: wk mcp servers update <handle>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			opts := map[string]any{}
			if cmd.Flags().Changed("name") {
				opts["name"] = name
			}
			if cmd.Flags().Changed("description") {
				opts["description"] = description
			}
			if cmd.Flags().Changed("auth-type") {
				opts["auth_type"] = authType
			}
			if cmd.Flags().Changed("folder-id") {
				opts["folder_id"] = folderID
			}
			if len(opts) == 0 {
				return fmt.Errorf("at least one update flag is required (--name, --description, --auth-type, --folder-id)")
			}

			srv, err := client.MCPServers().Update(cmd.Context(), args[0], opts)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, srv)
			}

			fmt.Fprintf(os.Stderr, "Updated MCP server %q (ID: %s)\n", srv.Name, srv.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Server name")
	cmd.Flags().StringVar(&description, "description", "", "Server description")
	cmd.Flags().StringVar(&authType, "auth-type", "", "Authentication type (token, workato_idp)")
	cmd.Flags().IntVar(&folderID, "folder-id", 0, "Folder ID")
	return cmd
}

func newMCPServersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <handle>",
		Short:   "Delete an MCP server",
		Example: `  wk mcp servers delete mcps-AYcNrsC8-Dd8-AB`,
		Args:    requireArgs(1, "server handle is required, e.g.: wk mcp servers delete <handle>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			if err := client.MCPServers().Delete(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Deleted MCP server %s\n", args[0])
			return nil
		},
	}
}

func newMCPServersTokenRenewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "token-renew <handle>",
		Short: "Renew the authentication token for an MCP server",
		Long: `Renew the authentication token, generating a new MCP URL with a fresh token.
The new URL is printed to stdout. The previous token is immediately invalidated.`,
		Example: `  wk mcp servers token-renew mcps-AYcNrsC8-Dd8-AB
  wk mcp servers token-renew mcps-AYcNrsC8-Dd8-AB --json`,
		Args: requireArgs(1, "server handle is required, e.g.: wk mcp servers token-renew <handle>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			srv, err := client.MCPServers().TokenRenew(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, srv)
			}

			fmt.Fprintf(os.Stderr, "Renewed token for MCP server %q (ID: %s)\n", srv.Name, srv.ID)
			fmt.Fprintln(os.Stdout, srv.MCPURL)
			return nil
		},
	}
}

func newMCPServersToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage tools assigned to an MCP server",
	}
	cmd.AddCommand(newMCPServersToolsListCmd())
	cmd.AddCommand(newMCPServersToolsAddCmd())
	cmd.AddCommand(newMCPServersToolsRemoveCmd())
	cmd.AddCommand(newMCPServersToolsUpdateCmd())
	return cmd
}

func newMCPServersToolsListCmd() *cobra.Command {
	var page, perPage int

	cmd := &cobra.Command{
		Use:   "list <handle>",
		Short: "List tools assigned to an MCP server",
		Example: `  wk mcp servers tools list mcps-AYcNrsC8-Dd8-AB
  wk mcp servers tools list mcps-AYcNrsC8-Dd8-AB --json`,
		Args: requireArgs(1, "server handle is required, e.g.: wk mcp servers tools list <handle>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			opts := &api.PaginationOptions{Page: page, PerPage: perPage}
			tools, err := client.MCPServers().ListTools(cmd.Context(), args[0], opts)
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "DESCRIPTION", "FLOW ID", "ACTIVE"}
			var rows [][]string
			for _, t := range tools {
				desc := ""
				if t.Description != nil {
					desc = *t.Description
					if len(desc) > 60 {
						desc = desc[:57] + "..."
					}
				}
				active := "no"
				if t.Active {
					active = "yes"
				}
				rows = append(rows, []string{
					strconv.Itoa(t.ID),
					t.Name,
					desc,
					strconv.Itoa(t.FlowID),
					active,
				})
			}
			meta := output.PageMeta{Page: page, PerPage: perPage, HasNext: perPage > 0 && len(tools) == perPage}
			return rctx.Formatter.FormatPage(os.Stdout, tools, headers, rows, meta)
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "Items per page (max 100)")
	return cmd
}

func newMCPServersToolsAddCmd() *cobra.Command {
	var recipes []int

	cmd := &cobra.Command{
		Use:   "add <handle>",
		Short: "Assign recipes as tools to an MCP server",
		Example: `  wk mcp servers tools add mcps-AYcNrsC8-Dd8-AB --recipe 277601
  wk mcp servers tools add mcps-AYcNrsC8-Dd8-AB --recipe 277601 --recipe 277602`,
		Args: requireArgs(1, "server handle is required, e.g.: wk mcp servers tools add <handle> --recipe <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}
			if len(recipes) == 0 {
				return fmt.Errorf("at least one --recipe is required")
			}

			tools, err := buildRecipeTools(cmd.Context(), client, recipes)
			if err != nil {
				return err
			}
			if err := client.MCPServers().AssignTools(cmd.Context(), args[0], tools); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Assigned %d recipe tool(s) to MCP server %s\n", len(tools), args[0])
			return nil
		},
	}

	cmd.Flags().IntSliceVar(&recipes, "recipe", nil, "Recipe ID to assign as a tool (repeatable)")
	return cmd
}

func newMCPServersToolsRemoveCmd() *cobra.Command {
	var toolID, recipeID int

	cmd := &cobra.Command{
		Use:   "remove <handle>",
		Short: "Remove a tool from an MCP server",
		Long: `Remove an assigned tool from an MCP server.

Identify the tool either by its tool ID (--tool, from "tools list") or by the
backing recipe ID (--recipe, resolved via the tool's flow_id). Only tools
backed by recipe functions or genies can be removed.`,
		Example: `  wk mcp servers tools remove mcps-AYcNrsC8-Dd8-AB --tool 5
  wk mcp servers tools remove mcps-AYcNrsC8-Dd8-AB --recipe 277601`,
		Args: requireArgs(1, "server handle is required, e.g.: wk mcp servers tools remove <handle> --tool <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}
			handle := args[0]

			setTool := cmd.Flags().Changed("tool")
			setRecipe := cmd.Flags().Changed("recipe")
			if setTool == setRecipe {
				return fmt.Errorf("exactly one of --tool or --recipe is required")
			}

			id := toolID
			if setRecipe {
				// Resolve the recipe to its assigned tool via flow_id.
				tools, lerr := client.MCPServers().ListTools(cmd.Context(), handle, &api.PaginationOptions{PerPage: 100})
				if lerr != nil {
					return lerr
				}
				found := false
				for _, t := range tools {
					if t.FlowID == recipeID {
						id = t.ID
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("no tool backed by recipe %d found on server %s", recipeID, handle)
				}
			}

			if err := client.MCPServers().DeleteTool(cmd.Context(), handle, id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Removed tool %d from MCP server %s\n", id, handle)
			return nil
		},
	}

	cmd.Flags().IntVar(&toolID, "tool", 0, "Tool ID (from `tools list`)")
	cmd.Flags().IntVar(&recipeID, "recipe", 0, "Backing recipe ID (resolved to its tool)")
	return cmd
}

func newMCPServersToolsUpdateCmd() *cobra.Command {
	var toolID int
	var description string

	cmd := &cobra.Command{
		Use:     "update <handle>",
		Short:   "Update an assigned tool (e.g. its description)",
		Example: `  wk mcp servers tools update mcps-AYcNrsC8-Dd8-AB --tool 5 --description "Creates a Salesforce lead"`,
		Args:    requireArgs(1, "server handle is required, e.g.: wk mcp servers tools update <handle> --tool <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("tool") {
				return fmt.Errorf("--tool is required")
			}

			opts := map[string]any{}
			if cmd.Flags().Changed("description") {
				opts["description"] = description
			}
			if len(opts) == 0 {
				return fmt.Errorf("at least one update flag is required (--description)")
			}

			tool, err := client.MCPServers().UpdateTool(cmd.Context(), args[0], toolID, opts)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, tool)
			}

			fmt.Fprintf(os.Stderr, "Updated tool %d on MCP server %s\n", toolID, args[0])
			return nil
		},
	}

	cmd.Flags().IntVar(&toolID, "tool", 0, "Tool ID to update (from `tools list`)")
	cmd.Flags().StringVar(&description, "description", "", "New tool description")
	return cmd
}

type mcpServerDef struct {
	Name          string `json:"name"`
	APICollection string `json:"api_collection"`
	Description   string `json:"description"`
	Persona       string `json:"persona,omitempty"`
}

func newMCPServersCreateBatchCmd() *cobra.Command {
	var fromFile, fromCSV string
	var folderID int
	var dryRun, continueOnError bool

	cmd := &cobra.Command{
		Use:   "create-batch",
		Short: "Create MCP servers in batch from a manifest or CSV",
		Long: `Create multiple MCP servers from a JSON manifest or CSV file.

JSON manifest: {"servers": [{"name": "...", "api_collection": "...", "description": "..."}]}
CSV columns: name,api_collection,description,persona

The api_collection field is resolved by name (or numeric ID) to link the server to an existing
API collection, giving it tools automatically.`,
		Example: `  wk mcp servers create-batch --from-file mcp-servers.json --folder-id 42
  wk mcp servers create-batch --from-csv mcp_servers.csv --folder-id 42
  wk mcp servers create-batch --from-file mcp-servers.json --folder-id 42 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			if !cmd.Flags().Changed("folder-id") {
				return fmt.Errorf("--folder-id is required")
			}

			if fromFile == "" && fromCSV == "" {
				return fmt.Errorf("--from-file or --from-csv is required")
			}

			var defs []mcpServerDef

			if fromFile != "" {
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("reading manifest: %w", err)
				}
				var manifest struct {
					Servers []mcpServerDef `json:"servers"`
				}
				if err := json.Unmarshal(data, &manifest); err != nil {
					return fmt.Errorf("parsing manifest: %w", err)
				}
				defs = manifest.Servers
			} else {
				parsed, err := parseMCPServerCSV(fromCSV)
				if err != nil {
					return err
				}
				defs = parsed
			}

			if len(defs) == 0 {
				return fmt.Errorf("no server definitions found")
			}

			type resultEntry struct {
				Name   string `json:"name"`
				ID     string `json:"id,omitempty"`
				MCPURL string `json:"mcp_url,omitempty"`
				Error  string `json:"error,omitempty"`
			}

			collectionCache := make(map[string]int)

			var created, failed []resultEntry

			for _, def := range defs {
				var assetID *int
				if def.APICollection != "" {
					id, err := resolveMCPCollectionCached(cmd.Context(), client, def.APICollection, collectionCache)
					if err != nil {
						if continueOnError {
							fmt.Fprintf(os.Stderr, "FAIL  %s: %v\n", def.Name, err)
							failed = append(failed, resultEntry{Name: def.Name, Error: err.Error()})
							continue
						}
						return fmt.Errorf("%s: %w", def.Name, err)
					}
					assetID = &id
				}

				if dryRun {
					collInfo := ""
					if assetID != nil {
						collInfo = fmt.Sprintf(", collection %d", *assetID)
					}
					fmt.Fprintf(os.Stderr, "[dry-run] would create: %s (folder %d%s)\n", def.Name, folderID, collInfo)
					continue
				}

				srv, err := client.MCPServers().Create(cmd.Context(), def.Name, folderID, def.Description, assetID)
				if err != nil {
					if continueOnError {
						fmt.Fprintf(os.Stderr, "FAIL  %s: %v\n", def.Name, err)
						failed = append(failed, resultEntry{Name: def.Name, Error: err.Error()})
						continue
					}
					return fmt.Errorf("creating %s: %w", def.Name, err)
				}

				fmt.Fprintf(os.Stderr, "OK    %s (ID: %s, tools: %d)\n", srv.Name, srv.ID, srv.ToolsCount)
				created = append(created, resultEntry{Name: srv.Name, ID: srv.ID, MCPURL: srv.MCPURL})
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "\n%d server(s) would be created\n", len(defs))
				return nil
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, map[string]any{
					"created": created,
					"failed":  failed,
				})
			}

			fmt.Fprintf(os.Stderr, "\n%d created, %d failed\n", len(created), len(failed))
			if len(failed) > 0 {
				return fmt.Errorf("batch completed with %d error(s)", len(failed))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fromFile, "from-file", "", "JSON manifest file")
	cmd.Flags().StringVar(&fromCSV, "from-csv", "", "CSV file with server definitions")
	cmd.Flags().IntVar(&folderID, "folder-id", 0, "Folder ID for all servers (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be created without making API calls")
	cmd.Flags().BoolVar(&continueOnError, "continue-on-error", false, "Continue creating remaining servers if one fails")
	return cmd
}

func parseMCPServerCSV(csvPath string) ([]mcpServerDef, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("opening CSV: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must have a header row and at least one data row")
	}

	header := records[0]
	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	if _, ok := colIdx["name"]; !ok {
		return nil, fmt.Errorf("CSV missing required column 'name' (have: %s)", strings.Join(header, ", "))
	}

	var defs []mcpServerDef
	for _, row := range records[1:] {
		def := mcpServerDef{
			Name: strings.TrimSpace(row[colIdx["name"]]),
		}
		if idx, ok := colIdx["api_collection"]; ok && idx < len(row) {
			def.APICollection = strings.TrimSpace(row[idx])
		}
		if idx, ok := colIdx["description"]; ok && idx < len(row) {
			def.Description = strings.TrimSpace(row[idx])
		}
		if idx, ok := colIdx["persona"]; ok && idx < len(row) {
			def.Persona = strings.TrimSpace(row[idx])
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// resolveMCPCollectionCached resolves a collection name or ID with caching
// across batch iterations to avoid repeated API calls.
func resolveMCPCollectionCached(ctx context.Context, client api.Client, value string, cache map[string]int) (int, error) {
	if cached, ok := cache[value]; ok {
		return cached, nil
	}
	id, err := resolveMCPCollection(ctx, client, value)
	if err != nil {
		return 0, err
	}
	cache[value] = id
	return id, nil
}

func newMCPServersPoliciesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policies",
		Short: "Read or update an MCP server's rate/quota/IP policy",
	}
	cmd.AddCommand(newMCPServersPoliciesGetCmd())
	cmd.AddCommand(newMCPServersPoliciesSetCmd())
	return cmd
}

func newMCPServersPoliciesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <handle>",
		Short:   "Get an MCP server's policy",
		Example: `  wk mcp servers policies get mcps-AYcNrsC8-Dd8-AB --json`,
		Args:    requireArgs(1, "server handle is required, e.g.: wk mcp servers policies get <handle>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			policy, err := client.MCPServers().GetServerPolicies(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, policy)
			}

			fmt.Fprintf(os.Stdout, "Rate limits:  %v\n", policy.RateLimits)
			fmt.Fprintf(os.Stdout, "Quota limits: %v\n", policy.QuotaLimits)
			fmt.Fprintf(os.Stdout, "IP allow:     %v\n", policy.IPAllowList)
			fmt.Fprintf(os.Stdout, "IP deny:      %v\n", policy.IPDenyList)
			return nil
		},
	}
}

func newMCPServersPoliciesSetCmd() *cobra.Command {
	var ratePerMinute, quotaPerDay int
	var ipAllow, ipDeny []string

	cmd := &cobra.Command{
		Use:   "set <handle>",
		Short: "Update an MCP server's policy",
		Long: `Update an MCP server's rate/quota/IP policy.

Only the flags you set are sent; unset fields are left unchanged.`,
		Example: `  wk mcp servers policies set mcps-AYcNrsC8-Dd8-AB --rate-per-minute 30 --quota-per-day 5000
  wk mcp servers policies set mcps-AYcNrsC8-Dd8-AB --ip-allow 203.0.113.0/24,198.51.100.42`,
		Args: requireArgs(1, "server handle is required, e.g.: wk mcp servers policies set <handle>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			policy := map[string]any{}
			if cmd.Flags().Changed("rate-per-minute") {
				policy["rate_limits"] = map[string]int{"per_minute": ratePerMinute}
			}
			if cmd.Flags().Changed("quota-per-day") {
				policy["quota_limits"] = map[string]int{"per_day": quotaPerDay}
			}
			if cmd.Flags().Changed("ip-allow") {
				policy["ip_allow_list"] = ipAllow
			}
			if cmd.Flags().Changed("ip-deny") {
				policy["ip_deny_list"] = ipDeny
			}
			if len(policy) == 0 {
				return fmt.Errorf("at least one policy flag is required (--rate-per-minute, --quota-per-day, --ip-allow, --ip-deny)")
			}

			updated, err := client.MCPServers().SetServerPolicies(cmd.Context(), args[0], policy)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, updated)
			}

			fmt.Fprintf(os.Stderr, "Updated policy for MCP server %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().IntVar(&ratePerMinute, "rate-per-minute", 0, "Requests allowed per minute")
	cmd.Flags().IntVar(&quotaPerDay, "quota-per-day", 0, "Requests allowed per day")
	cmd.Flags().StringSliceVar(&ipAllow, "ip-allow", nil, "IP allow list (CIDRs, comma-separated)")
	cmd.Flags().StringSliceVar(&ipDeny, "ip-deny", nil, "IP deny list (CIDRs, comma-separated)")
	return cmd
}

func newMCPServersUserGroupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "user-groups",
		Aliases: []string{"user-group"},
		Short:   "Grant or revoke IdP user group access to an MCP server",
	}
	cmd.AddCommand(newMCPServersUserGroupsAddCmd())
	cmd.AddCommand(newMCPServersUserGroupsRemoveCmd())
	return cmd
}

func newMCPServersUserGroupsAddCmd() *cobra.Command {
	var groups []string

	cmd := &cobra.Command{
		Use:   "add <handle>",
		Short: "Grant IdP user groups access to an MCP server",
		Example: `  wk mcp servers user-groups add mcps-AYcNrsC8-Dd8-AB --group group-abc123
  wk mcp servers user-groups add mcps-AYcNrsC8-Dd8-AB --group group-abc123 --group group-def456`,
		Args: requireArgs(1, "server handle is required, e.g.: wk mcp servers user-groups add <handle> --group <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}
			if len(groups) == 0 {
				return fmt.Errorf("at least one --group is required")
			}

			if err := client.MCPServers().AssignUserGroups(cmd.Context(), args[0], groups); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Granted %d user group(s) access to MCP server %s\n", len(groups), args[0])
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&groups, "group", nil, "IdP user group ID (repeatable)")
	return cmd
}

func newMCPServersUserGroupsRemoveCmd() *cobra.Command {
	var groups []string

	cmd := &cobra.Command{
		Use:     "remove <handle>",
		Short:   "Revoke IdP user groups from an MCP server",
		Example: `  wk mcp servers user-groups remove mcps-AYcNrsC8-Dd8-AB --group group-abc123`,
		Args:    requireArgs(1, "server handle is required, e.g.: wk mcp servers user-groups remove <handle> --group <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}
			if len(groups) == 0 {
				return fmt.Errorf("at least one --group is required")
			}

			if err := client.MCPServers().RemoveUserGroups(cmd.Context(), args[0], groups); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Revoked %d user group(s) from MCP server %s\n", len(groups), args[0])
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&groups, "group", nil, "IdP user group ID (repeatable)")
	return cmd
}

func newMCPUserGroupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "user-groups",
		Aliases: []string{"user-group"},
		Short:   "Manage identity-provider user groups",
	}
	cmd.AddCommand(newMCPUserGroupsListCmd())
	return cmd
}

func newMCPUserGroupsListCmd() *cobra.Command {
	var page, perPage int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List IdP user groups (resolve IDs for server user-group grants)",
		Example: `  wk mcp user-groups list
  wk mcp user-groups list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			opts := &api.PaginationOptions{Page: page, PerPage: perPage}
			groups, err := client.MCPServers().ListUserGroups(cmd.Context(), opts)
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "USERS"}
			var rows [][]string
			for _, g := range groups {
				rows = append(rows, []string{g.ID, g.Name, strconv.Itoa(g.UsersCount)})
			}
			meta := output.PageMeta{Page: page, PerPage: perPage, HasNext: perPage > 0 && len(groups) == perPage}
			return rctx.Formatter.FormatPage(os.Stdout, groups, headers, rows, meta)
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "Items per page (max 100)")
	return cmd
}
