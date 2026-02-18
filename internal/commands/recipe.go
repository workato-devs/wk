package commands

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/api"
)

func newRecipesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "recipes",
		Aliases: []string{"recipe"},
		Short:   "Manage Workato recipes",
	}
	cmd.AddCommand(newRecipesListCmd())
	cmd.AddCommand(newRecipesGetCmd())
	cmd.AddCommand(newRecipesStartCmd())
	cmd.AddCommand(newRecipesStopCmd())
	cmd.AddCommand(newRecipesExportCmd())
	cmd.AddCommand(newRecipesImportCmd())
	return cmd
}

func newRecipesListCmd() *cobra.Command {
	var folderID int
	var status string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recipes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			opts := &api.RecipeListOptions{
				Status: status,
			}
			if cmd.Flags().Changed("folder") {
				opts.FolderID = &folderID
			}

			recipes, err := client.Recipes().List(cmd.Context(), opts)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, recipes)
			}

			headers := []string{"ID", "NAME", "FOLDER", "RUNNING", "VERSION"}
			var rows [][]string
			for _, r := range recipes {
				running := "stopped"
				if r.Running {
					running = "running"
				}
				rows = append(rows, []string{
					strconv.Itoa(r.ID),
					r.Name,
					strconv.Itoa(r.FolderID),
					running,
					strconv.Itoa(r.Version),
				})
			}
			return rctx.Formatter.FormatList(os.Stdout, headers, rows)
		},
	}

	cmd.Flags().IntVar(&folderID, "folder", 0, "Filter by folder ID")
	cmd.Flags().StringVar(&status, "status", "all", "Filter by status (running, stopped, all)")
	return cmd
}

func newRecipesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get recipe details",
		Args:  cobra.ExactArgs(1),
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
				return fmt.Errorf("invalid recipe ID: %s", args[0])
			}

			recipe, err := client.Recipes().Get(cmd.Context(), id)
			if err != nil {
				return err
			}

			if !flagJSON {
				running := "no"
				if recipe.Running {
					running = "yes"
				}
				fmt.Fprintf(os.Stdout, "ID:          %d\n", recipe.ID)
				fmt.Fprintf(os.Stdout, "Name:        %s\n", recipe.Name)
				fmt.Fprintf(os.Stdout, "Description: %s\n", recipe.Description)
				fmt.Fprintf(os.Stdout, "Folder ID:   %d\n", recipe.FolderID)
				fmt.Fprintf(os.Stdout, "Running:     %s\n", running)
				fmt.Fprintf(os.Stdout, "Version:     %d\n", recipe.Version)
				fmt.Fprintf(os.Stdout, "Updated:     %s\n", recipe.UpdatedAt.Format(time.RFC3339))
				return nil
			}
			return rctx.Formatter.Format(os.Stdout, recipe)
		},
	}
}

func newRecipesStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id>",
		Short: "Start a recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid recipe ID: %s", args[0])
			}

			if err := client.Recipes().Start(cmd.Context(), id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "Recipe %d started\n", id)
			return nil
		},
	}
}

func newRecipesStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid recipe ID: %s", args[0])
			}

			if err := client.Recipes().Stop(cmd.Context(), id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "Recipe %d stopped\n", id)
			return nil
		},
	}
}

func newRecipesExportCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "export <id>",
		Short: "Export a recipe as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid recipe ID: %s", args[0])
			}

			data, err := client.Recipes().Export(cmd.Context(), id)
			if err != nil {
				return err
			}

			if outputFile != "" {
				if err := os.WriteFile(outputFile, data, 0644); err != nil {
					return fmt.Errorf("writing file: %w", err)
				}
				fmt.Fprintf(os.Stdout, "Recipe %d exported to %s\n", id, outputFile)
				return nil
			}

			_, err = os.Stdout.Write(data)
			return err
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path")
	return cmd
}

func newRecipesImportCmd() *cobra.Command {
	var folderID int

	cmd := &cobra.Command{
		Use:   "import <path>",
		Short: "Import a recipe from JSON file",
		Args:  cobra.ExactArgs(1),
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

			recipe, err := client.Recipes().Import(cmd.Context(), folderID, data)
			if err != nil {
				return err
			}

			if !flagJSON {
				running := "no"
				if recipe.Running {
					running = "yes"
				}
				fmt.Fprintf(os.Stdout, "ID:          %d\n", recipe.ID)
				fmt.Fprintf(os.Stdout, "Name:        %s\n", recipe.Name)
				fmt.Fprintf(os.Stdout, "Folder ID:   %d\n", recipe.FolderID)
				fmt.Fprintf(os.Stdout, "Running:     %s\n", running)
				return nil
			}
			return rctx.Formatter.Format(os.Stdout, recipe)
		},
	}

	cmd.Flags().IntVar(&folderID, "folder", 0, "Target folder ID")
	return cmd
}
