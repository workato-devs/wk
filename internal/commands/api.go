package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/api"
	"github.com/workato-devs/wk-cli-beta/internal/output"
)

func newAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Manage API Platform resources",
	}
	cmd.AddCommand(newAPICollectionsCmd())
	cmd.AddCommand(newAPIEndpointsCmd())
	cmd.AddCommand(newAPIClientsCmd())
	return cmd
}

func newAPICollectionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "collections",
		Aliases: []string{"collection"},
		Short:   "Manage API collections",
	}
	cmd.AddCommand(newAPICollectionsListCmd())
	cmd.AddCommand(newAPICollectionsCreateCmd())
	return cmd
}

func newAPICollectionsListCmd() *cobra.Command {
	var page, perPage int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API collections",
		Example: `  wk api collections list
  wk api collections list --page 1 --per-page 20 --json`,
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
			collections, err := client.APICollections().List(cmd.Context(), opts)
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "VERSION", "URL", "PROJECT ID"}
			var rows [][]string
			for _, c := range collections {
				pid := ""
				if c.ProjectID != nil {
					pid = strconv.Itoa(*c.ProjectID)
				}
				rows = append(rows, []string{
					strconv.Itoa(c.ID),
					c.Name,
					c.Version,
					c.URL,
					pid,
				})
			}
			meta := output.PageMeta{Page: page, PerPage: perPage, HasNext: perPage > 0 && len(collections) == perPage}
			return rctx.Formatter.FormatPage(os.Stdout, collections, headers, rows, meta)
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "Items per page")
	return cmd
}

func newAPICollectionsCreateCmd() *cobra.Command {
	var name string
	var projectID int
	var fromFile string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an API collection",
		Long: `Create an API collection.

When --from-file is used, the JSON file provides the collection name.
Flags (--name, --project) override file values.`,
		Example: `  wk api collections create --name "Customer API" --json
  wk api collections create --name "Customer API" --project 123 --json
  wk api collections create --from-file collection.json --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			if fromFile != "" {
				data, ferr := os.ReadFile(fromFile)
				if ferr != nil {
					return fmt.Errorf("reading file: %w", ferr)
				}
				var fileDef struct {
					Name      string `json:"name"`
					ProjectID *int   `json:"project_id,omitempty"`
				}
				if jerr := json.Unmarshal(data, &fileDef); jerr != nil {
					return fmt.Errorf("invalid JSON: %w", jerr)
				}
				if name == "" {
					name = fileDef.Name
				}
				if !cmd.Flags().Changed("project") && fileDef.ProjectID != nil {
					projectID = *fileDef.ProjectID
				}
			}

			if name == "" {
				return fmt.Errorf("--name is required (or use --from-file with a JSON file containing \"name\")")
			}

			var pid *int
			if cmd.Flags().Changed("project") || projectID != 0 {
				pid = &projectID
			}

			collection, err := client.APICollections().Create(cmd.Context(), name, pid)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, collection)
			}

			fmt.Fprintf(os.Stderr, "Created API collection %q (ID: %d)\n", collection.Name, collection.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Collection name")
	cmd.Flags().IntVar(&projectID, "project", 0, "Project ID")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "JSON file with collection definition")
	return cmd
}

func newAPIEndpointsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "endpoints",
		Aliases: []string{"endpoint"},
		Short:   "Manage API endpoints",
	}
	cmd.AddCommand(newAPIEndpointsListCmd())
	cmd.AddCommand(newAPIEndpointsCreateCmd())
	cmd.AddCommand(newAPIEndpointsEnableCmd())
	cmd.AddCommand(newAPIEndpointsDisableCmd())
	return cmd
}

func newAPIEndpointsListCmd() *cobra.Command {
	var collectionID, page, perPage int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API endpoints",
		Example: `  wk api endpoints list
  wk api endpoints list --collection 42 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			var cid *int
			if cmd.Flags().Changed("collection") {
				cid = &collectionID
			}
			opts := &api.PaginationOptions{Page: page, PerPage: perPage}

			endpoints, err := client.APIEndpoints().List(cmd.Context(), cid, opts)
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "METHOD", "PATH", "RECIPE ID", "COLLECTION ID", "ACTIVE"}
			var rows [][]string
			for _, e := range endpoints {
				active := "no"
				if e.Active {
					active = "yes"
				}
				rows = append(rows, []string{
					strconv.Itoa(e.ID),
					e.Name,
					e.Method,
					e.Path,
					strconv.Itoa(e.FlowID),
					strconv.Itoa(e.APICollectionID),
					active,
				})
			}
			meta := output.PageMeta{Page: page, PerPage: perPage, HasNext: perPage > 0 && len(endpoints) == perPage}
			return rctx.Formatter.FormatPage(os.Stdout, endpoints, headers, rows, meta)
		},
	}

	cmd.Flags().IntVar(&collectionID, "collection", 0, "Filter by collection ID")
	cmd.Flags().IntVar(&page, "page", 0, "Page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "Items per page")
	return cmd
}

func newAPIEndpointsCreateCmd() *cobra.Command {
	var collectionID int

	cmd := &cobra.Command{
		Use:   "create <path>",
		Short: "Create an API endpoint from a JSON file",
		Long: `Create an API endpoint from a JSON definition file.

The file should contain: name, method, path, and either flow_id (recipe ID)
or recipe_name (resolved to flow_id via recipe lookup).`,
		Example: `  wk api endpoints create endpoint.json --collection 42
  wk api endpoints create endpoint.json --collection 42 --json`,
		Args: requireArgs(1, "file path is required, e.g.: wk api endpoints create <path> --collection <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}

			data, err = resolveRecipeNameInEndpointJSON(cmd.Context(), client, data)
			if err != nil {
				return err
			}

			ep, err := client.APIEndpoints().Create(cmd.Context(), collectionID, data)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, ep)
			}

			fmt.Fprintf(os.Stdout, "ID:            %d\n", ep.ID)
			fmt.Fprintf(os.Stdout, "Name:          %s\n", ep.Name)
			fmt.Fprintf(os.Stdout, "Method:        %s\n", ep.Method)
			fmt.Fprintf(os.Stdout, "Path:          %s\n", ep.Path)
			fmt.Fprintf(os.Stdout, "Collection ID: %d\n", ep.APICollectionID)
			fmt.Fprintf(os.Stdout, "Recipe ID:     %d\n", ep.FlowID)
			if ep.URL != "" {
				fmt.Fprintf(os.Stdout, "URL:           %s\n", ep.URL)
			}
			if ep.Description != nil && *ep.Description != "" {
				fmt.Fprintf(os.Stdout, "Description:   %s\n", *ep.Description)
			}
			active := "no"
			if ep.Active {
				active = "yes"
			}
			fmt.Fprintf(os.Stdout, "Active:        %s\n", active)
			return nil
		},
	}

	cmd.Flags().IntVar(&collectionID, "collection", 0, "API collection ID")
	_ = cmd.MarkFlagRequired("collection")
	return cmd
}

func newAPIEndpointsEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "enable <id>",
		Short:   "Enable an API endpoint",
		Example: `  wk api endpoints enable 789`,
		Args:  requireArgs(1, "endpoint ID is required, e.g.: wk api endpoints enable <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid endpoint ID: %s", args[0])
			}

			if err := client.APIEndpoints().Enable(cmd.Context(), id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Endpoint %d enabled\n", id)
			return nil
		},
	}
}

func newAPIEndpointsDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "disable <id>",
		Short:   "Disable an API endpoint",
		Example: `  wk api endpoints disable 789`,
		Args:  requireArgs(1, "endpoint ID is required, e.g.: wk api endpoints disable <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid endpoint ID: %s", args[0])
			}

			if err := client.APIEndpoints().Disable(cmd.Context(), id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Endpoint %d disabled\n", id)
			return nil
		},
	}
}

func newAPIClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clients",
		Aliases: []string{"client"},
		Short:   "Manage API Platform clients",
	}
	cmd.AddCommand(newAPIClientsListCmd())
	cmd.AddCommand(newAPIClientsGetCmd())
	cmd.AddCommand(newAPIClientsCreateCmd())
	cmd.AddCommand(newAPIClientsDeleteCmd())
	cmd.AddCommand(newAPIClientsKeysCmd())
	return cmd
}

func newAPIClientsListCmd() *cobra.Command {
	var page, perPage int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API Platform clients",
		Example: `  wk api clients list
  wk api clients list --page 1 --per-page 20 --json`,
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
			clients, err := client.APIClients().List(cmd.Context(), opts)
			if err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "AUTH TYPE", "COLLECTIONS", "ACTIVE KEYS"}
			var rows [][]string
			for _, c := range clients {
				var colNames []string
				for _, col := range c.APICollections {
					colNames = append(colNames, col.Name)
				}
				collections := ""
				if len(colNames) > 0 {
					collections = fmt.Sprintf("%v", colNames)
				}
				rows = append(rows, []string{
					strconv.Itoa(c.ID),
					c.Name,
					c.AuthType,
					collections,
					strconv.Itoa(c.ActiveAPIKeysCount),
				})
			}
			meta := output.PageMeta{Page: page, PerPage: perPage, HasNext: perPage > 0 && len(clients) == perPage}
			return rctx.Formatter.FormatPage(os.Stdout, clients, headers, rows, meta)
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number")
	cmd.Flags().IntVar(&perPage, "per-page", 0, "Items per page")
	return cmd
}

func newAPIClientsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get an API Platform client",
		Example: `  wk api clients get 42
  wk api clients get 42 --json`,
		Args: requireArgs(1, "client ID is required, e.g.: wk api clients get <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid client ID: %s", args[0])
			}

			ac, err := client.APIClients().Get(cmd.Context(), id)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, ac)
			}

			fmt.Fprintf(os.Stdout, "ID:          %d\n", ac.ID)
			fmt.Fprintf(os.Stdout, "Name:        %s\n", ac.Name)
			fmt.Fprintf(os.Stdout, "Auth Type:   %s\n", ac.AuthType)
			fmt.Fprintf(os.Stdout, "Legacy:      %v\n", ac.IsLegacy)
			fmt.Fprintf(os.Stdout, "mTLS:        %v\n", ac.MTLSEnabled)
			fmt.Fprintf(os.Stdout, "Active Keys: %d\n", ac.ActiveAPIKeysCount)
			fmt.Fprintf(os.Stdout, "Total Keys:  %d\n", ac.TotalAPIKeysCount)
			if len(ac.APICollections) > 0 {
				fmt.Fprintf(os.Stdout, "Collections:\n")
				for _, col := range ac.APICollections {
					fmt.Fprintf(os.Stdout, "  - %s (ID: %d)\n", col.Name, col.ID)
				}
			}
			if len(ac.APIKeys) > 0 {
				fmt.Fprintf(os.Stdout, "Keys:\n")
				for _, k := range ac.APIKeys {
					active := "inactive"
					if k.Active {
						active = "active"
					}
					fmt.Fprintf(os.Stdout, "  - %s (ID: %d, %s)\n", k.Name, k.ID, active)
				}
			}
			return nil
		},
	}
}

func newAPIClientsCreateCmd() *cobra.Command {
	var name, authType string
	var collections []string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an API Platform client",
		Example: `  wk api clients create --name "My Client" --collections 42,43
  wk api clients create --name "My Client" --auth-type token --json`,
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

			if authType == "" {
				authType = "token"
			}

			ac, err := client.APIClients().Create(cmd.Context(), name, collections, authType)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, ac)
			}

			fmt.Fprintf(os.Stderr, "Created API client %q (ID: %d)\n", ac.Name, ac.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Client name")
	cmd.Flags().StringSliceVar(&collections, "collections", nil, "API collection IDs (as strings, e.g. 42,43)")
	cmd.Flags().StringVar(&authType, "auth-type", "token", "Authentication type")
	return cmd
}

func newAPIClientsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete an API Platform client",
		Example: `  wk api clients delete 42`,
		Args:    requireArgs(1, "client ID is required, e.g.: wk api clients delete <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid client ID: %s", args[0])
			}

			if err := client.APIClients().Delete(cmd.Context(), id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Deleted API client %d\n", id)
			return nil
		},
	}
}

func newAPIClientsKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keys",
		Aliases: []string{"key"},
		Short:   "Manage API client keys",
	}
	cmd.AddCommand(newAPIClientsKeysCreateCmd())
	cmd.AddCommand(newAPIClientsKeysRefreshCmd())
	return cmd
}

func newAPIClientsKeysCreateCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create <client-id>",
		Short: "Create an API key for a client",
		Long: `Create an API key for a client. The auth token is printed to stdout
and is only visible at creation time — it cannot be retrieved later.`,
		Example: `  wk api clients keys create 42 --name "prod-key"
  wk api clients keys create 42 --name "prod-key" --json`,
		Args: requireArgs(1, "client ID is required, e.g.: wk api clients keys create <client-id> --name <name>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			clientID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid client ID: %s", args[0])
			}

			if name == "" {
				return fmt.Errorf("--name is required")
			}

			key, err := client.APIClients().CreateKey(cmd.Context(), clientID, name)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, key)
			}

			fmt.Fprintf(os.Stderr, "Created API key %q (ID: %d) for client %d\n", key.Name, key.ID, clientID)
			fmt.Fprintln(os.Stdout, key.AuthToken)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Key name")
	return cmd
}

func newAPIClientsKeysRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh <client-id> <key-id>",
		Short: "Rotate an API key (generates a new auth token)",
		Long: `Rotate an API key, generating a new auth token. The new token is printed
to stdout and is only visible at this moment — it cannot be retrieved later.
The previous token is immediately invalidated.`,
		Example: `  wk api clients keys refresh 42 10
  wk api clients keys refresh 42 10 --json`,
		Args: requireArgs(2, "client ID and key ID are required, e.g.: wk api clients keys refresh <client-id> <key-id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			clientID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid client ID: %s", args[0])
			}
			keyID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid key ID: %s", args[1])
			}

			key, err := client.APIClients().RefreshKey(cmd.Context(), clientID, keyID)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, key)
			}

			fmt.Fprintf(os.Stderr, "Rotated API key %q (ID: %d) for client %d\n", key.Name, key.ID, clientID)
			fmt.Fprintln(os.Stdout, key.AuthToken)
			return nil
		},
	}
}

// resolveRecipeNameInEndpointJSON checks if the JSON body contains
// "recipe_name" and resolves it to "flow_id" via recipe list lookup.
// Returns the modified JSON with flow_id set and recipe_name removed.
func resolveRecipeNameInEndpointJSON(ctx context.Context, client api.Client, data []byte) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return data, nil
	}

	recipeName, hasName := body["recipe_name"].(string)
	_, hasFlowID := body["flow_id"]
	if !hasName {
		return data, nil
	}
	if hasFlowID {
		return nil, fmt.Errorf("cannot specify both flow_id and recipe_name")
	}

	recipes, err := client.Recipes().List(ctx, &api.RecipeListOptions{PerPage: 100})
	if err != nil {
		return nil, fmt.Errorf("looking up recipe %q: %w", recipeName, err)
	}

	var matches []api.Recipe
	for _, r := range recipes {
		if r.Name == recipeName {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no recipe found with name %q", recipeName)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple recipes match name %q (%d found); use flow_id instead", recipeName, len(matches))
	}

	body["flow_id"] = matches[0].ID
	delete(body, "recipe_name")
	return json.Marshal(body)
}

