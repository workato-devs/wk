package commands

import (
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

			headers := []string{"ID", "NAME", "HANDLE", "VERSION", "DESCRIPTION", "PROJECT ID"}
			var rows [][]string
			for _, c := range collections {
				rows = append(rows, []string{
					strconv.Itoa(c.ID),
					c.Name,
					c.Handle,
					c.Version,
					c.Description,
					strconv.Itoa(c.ProjectID),
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

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an API collection",
		Example: `  wk api collections create --name "Customer API" --project 123 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			collection, err := client.APICollections().Create(cmd.Context(), name, projectID)
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
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("project")
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
					strconv.Itoa(e.RecipeID),
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

The file should contain: name, method, path, and flow_id (recipe ID).`,
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
			fmt.Fprintf(os.Stdout, "Recipe ID:     %d\n", ep.RecipeID)
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

