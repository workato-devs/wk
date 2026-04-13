package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func newFoldersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "folders",
		Aliases: []string{"folder"},
		Short:   "Manage Workato folders",
	}
	cmd.AddCommand(newFoldersListCmd())
	cmd.AddCommand(newFoldersCreateCmd())
	cmd.AddCommand(newFoldersDeleteCmd())
	return cmd
}

func newFoldersListCmd() *cobra.Command {
	var parentID int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List folders",
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			var pid *int
			if cmd.Flags().Changed("parent") {
				pid = &parentID
			}

			folders, err := client.Folders().List(cmd.Context(), pid)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, folders)
			}

			headers := []string{"ID", "NAME", "PARENT ID"}
			var rows [][]string
			for _, f := range folders {
				parent := ""
				if f.ParentID != nil {
					parent = strconv.Itoa(*f.ParentID)
				}
				rows = append(rows, []string{
					strconv.Itoa(f.ID),
					f.Name,
					parent,
				})
			}
			return rctx.Formatter.FormatList(os.Stdout, headers, rows)
		},
	}

	cmd.Flags().IntVar(&parentID, "parent", 0, "Parent folder ID")
	return cmd
}

func newFoldersCreateCmd() *cobra.Command {
	var parentID int

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a folder",
		Args:  requireArgs(1, "folder name is required, e.g.: wk folders create <name>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			var pid *int
			if cmd.Flags().Changed("parent") {
				pid = &parentID
			}

			folder, err := client.Folders().Create(cmd.Context(), args[0], pid)
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, folder)
			}

			fmt.Fprintf(os.Stdout, "Created folder %q (ID: %d)\n", folder.Name, folder.ID)
			return nil
		},
	}

	cmd.Flags().IntVar(&parentID, "parent", 0, "Parent folder ID")
	return cmd
}

func newFoldersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a folder",
		Args:  requireArgs(1, "folder ID is required, e.g.: wk folders delete <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid folder ID: %s", args[0])
			}

			if err := client.Folders().Delete(cmd.Context(), id); err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "Folder %d deleted\n", id)
			return nil
		},
	}
}
