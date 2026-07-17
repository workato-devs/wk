package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
)

func newFoldersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "folders",
		Aliases: []string{"folder"},
		Short:   "Manage Workato folders",
	}
	cmd.AddCommand(newFoldersListCmd())
	cmd.AddCommand(newFoldersCreateCmd())
	cmd.AddCommand(newFoldersUpdateCmd())
	cmd.AddCommand(newFoldersDeleteCmd())
	return cmd
}

func newFoldersListCmd() *cobra.Command {
	var parentID int
	var projectsOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List folders",
		Example: `  wk folders list
  wk folders list --parent 123 --json
  wk folders list --projects`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			if projectsOnly && cmd.Flags().Changed("parent") {
				return fmt.Errorf("--projects and --parent are mutually exclusive (GET /projects is not scoped by parent)")
			}

			var folders []api.Folder
			if projectsOnly {
				// Projects have their own endpoint (GET /projects) rather
				// than being inferred from the folder list's is_project flag.
				folders, err = client.Folders().ListProjects(cmd.Context())
			} else {
				var pid *int
				if cmd.Flags().Changed("parent") {
					pid = &parentID
				}
				folders, err = client.Folders().List(cmd.Context(), pid)
			}
			if err != nil {
				return err
			}

			if projectsOnly {
				// A project IS a folder on the server — the top-level container
				// whose children are plain folders — so its constant identity is
				// its folder id, and project_id is an additive attribute. But
				// GET /projects reports each project's project_id as the object's
				// `id` and omits the folder id, which is the opposite of what
				// callers expect and NOT the id `wk folders update/delete` accept
				// (those take the folder id and route to the project endpoint via
				// is_project/project_id). Left as-is, copying an id from this list
				// — table or --json — into update/delete hits PUT /folders/{project_id}:
				// a 404, or worse a silent rename of an unrelated folder on an id
				// collision. Normalize every project so `id` is its folder id (the
				// constant identity) and `project_id` rides along, by cross-
				// referencing the root folder list, which carries both. This fixes
				// the table and the --json automation path from one source.
				byProjectID := map[int]int{}
				if roots, rerr := client.Folders().List(cmd.Context(), nil); rerr == nil {
					for _, r := range roots {
						if r.IsProject && r.ProjectID != 0 {
							byProjectID[r.ProjectID] = r.ID
						}
					}
				} else {
					fmt.Fprintf(os.Stderr, "warning: could not resolve folder ids for projects: %v\n", rerr)
				}
				for i := range folders {
					projectID := folders[i].ID
					folders[i].ProjectID = projectID
					folders[i].IsProject = true
					if fid, ok := byProjectID[projectID]; ok {
						folders[i].ID = fid
					} else {
						// Folder id unresolved: report 0 rather than passing the
						// project_id off as the folder id.
						folders[i].ID = 0
					}
				}
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, folders)
			}

			if projectsOnly {
				headers := []string{"FOLDER ID", "PROJECT ID", "NAME"}
				var rows [][]string
				for _, f := range folders {
					folderID := "—"
					if f.ID != 0 {
						folderID = strconv.Itoa(f.ID)
					}
					rows = append(rows, []string{
						folderID,
						strconv.Itoa(f.ProjectID),
						f.Name,
					})
				}
				return rctx.Formatter.FormatList(os.Stdout, headers, rows)
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
	cmd.Flags().BoolVar(&projectsOnly, "projects", false, "List projects via GET /projects instead of folders")
	return cmd
}

func newFoldersUpdateCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Rename a folder or project",
		Example: `  wk folders update 123 --name "New name"`,
		Long: `Rename a top-level Workato project or a nested folder. The Workato
API uses separate endpoints — PUT /projects/{id} for projects
(is_project=true), PUT /folders/{id} for plain folders. This
command lists top-level folders first and routes to the correct
endpoint based on the target's is_project flag (mirroring delete).`,
		Args: requireArgs(1, "folder ID is required, e.g.: wk folders update <id> --name <new>"),
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

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid folder ID: %s", args[0])
			}

			// Projects are always top-level; a single list call at the
			// workspace root resolves both is_project and project_id. If
			// the target isn't in the root list it's a nested folder,
			// which always routes to PUT /folders/{id}.
			topLevel, err := client.Folders().List(cmd.Context(), nil)
			if err != nil {
				return fmt.Errorf("listing top-level folders to determine type: %w", err)
			}
			var match *api.Folder
			for i, f := range topLevel {
				if f.ID == id {
					match = &topLevel[i]
					break
				}
			}

			var updated *api.Folder
			if match != nil && match.IsProject {
				// PUT /projects/{project_id} — not folder_id.
				if match.ProjectID == 0 {
					return fmt.Errorf("folder %d is a project but the API did not return project_id; cannot route update", id)
				}
				updated, err = client.Folders().UpdateProject(cmd.Context(), match.ProjectID, name)
			} else {
				updated, err = client.Folders().Update(cmd.Context(), id, name)
			}
			if err != nil {
				return err
			}

			if flagJSON {
				return rctx.Formatter.Format(os.Stdout, updated)
			}
			fmt.Fprintf(os.Stderr, "Renamed %d to %q\n", id, updated.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New name for the folder or project (required)")
	return cmd
}

func newFoldersCreateCmd() *cobra.Command {
	var parentID int

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a folder",
		Example: `  wk folders create "Marketing Recipes"
  wk folders create "Subfolder" --parent 123 --json`,
		Args: requireArgs(1, "folder name is required, e.g.: wk folders create <name>"),
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

			fmt.Fprintf(os.Stderr, "Created folder %q (ID: %d)\n", folder.Name, folder.ID)
			return nil
		},
	}

	cmd.Flags().IntVar(&parentID, "parent", 0, "Parent folder ID")
	return cmd
}

func newFoldersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a folder or project",
		Example: `  wk folders delete 123`,
		Long: `Delete a top-level Workato project or a nested folder. The Workato
API uses separate endpoints — DELETE /projects/{id} for projects
(is_project=true), DELETE /folders/{id} for plain folders. This
command lists top-level folders first and routes to the correct
endpoint based on the target's is_project flag.`,
		Args: requireArgs(1, "folder ID is required, e.g.: wk folders delete <id>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := resolveAPIClient(cmd)
			if err != nil {
				return err
			}

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid folder ID: %s", args[0])
			}

			// Projects are always top-level; a single list call at the
			// workspace root resolves both is_project and project_id. If
			// the target isn't in the root list it's a nested folder,
			// which always routes to DELETE /folders/{id}.
			topLevel, err := client.Folders().List(cmd.Context(), nil)
			if err != nil {
				return fmt.Errorf("listing top-level folders to determine type: %w", err)
			}
			var match *api.Folder
			for i, f := range topLevel {
				if f.ID == id {
					match = &topLevel[i]
					break
				}
			}

			if match != nil && match.IsProject {
				// DELETE /projects/{project_id} — not folder_id. The
				// project_id is a separate identifier returned on the
				// folder list response when is_project=true.
				if match.ProjectID == 0 {
					return fmt.Errorf("folder %d is a project but the API did not return project_id; cannot route delete", id)
				}
				if err := client.Folders().DeleteProject(cmd.Context(), match.ProjectID); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Project %d deleted (project_id=%d)\n", id, match.ProjectID)
				return nil
			}

			if err := client.Folders().Delete(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Folder %d deleted\n", id)
			return nil
		},
	}
}
