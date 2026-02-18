package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

func newInitCmd() *cobra.Command {
	var (
		flagName       string
		flagWorkspace  string
		flagServerPath string
		flagLocalPath  string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new wk project in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}

			// Check if wk.toml already exists in the current directory.
			configPath := filepath.Join(cwd, config.ProjectFile)
			if _, err := os.Stat(configPath); err == nil {
				return fmt.Errorf("wk.toml already exists. Use 'wk link' to update workspace.")
			}

			name := flagName
			workspace := flagWorkspace

			if flagJSON {
				// Non-interactive: require --name and --workspace.
				if name == "" {
					return fmt.Errorf("--name is required in non-interactive (--json) mode")
				}
				if workspace == "" {
					return fmt.Errorf("--workspace is required in non-interactive (--json) mode")
				}
			} else {
				// Interactive: prompt for missing values.
				reader := bufio.NewReader(os.Stdin)
				if name == "" {
					fmt.Print("Project name: ")
					name, _ = reader.ReadString('\n')
					name = strings.TrimSpace(name)
					if name == "" {
						return fmt.Errorf("project name is required")
					}
				}
				if workspace == "" {
					fmt.Print("Workspace profile: ")
					workspace, _ = reader.ReadString('\n')
					workspace = strings.TrimSpace(workspace)
					if workspace == "" {
						return fmt.Errorf("workspace profile is required")
					}
				}
			}

			cfg := &config.Config{
				Name:      name,
				Workspace: workspace,
			}

			if flagServerPath != "" {
				cfg.Sync = []config.SyncEntry{
					{
						ServerPath: flagServerPath,
						LocalPath:  flagLocalPath,
					},
				}
			}

			if err := config.Save(configPath, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			result := map[string]string{
				"status":    "initialized",
				"name":      name,
				"workspace": workspace,
				"path":      configPath,
			}

			if flagJSON {
				return rctx.Formatter.Format(cmd.OutOrStdout(), result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized wk project %q (workspace: %s) at %s\n", name, workspace, configPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagName, "name", "", "Project name")
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Workspace profile name")
	cmd.Flags().StringVar(&flagServerPath, "server-path", "", "Initial sync server path")
	cmd.Flags().StringVar(&flagLocalPath, "local-path", "./recipes", "Initial sync local path")

	return cmd
}
