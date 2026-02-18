package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

func newLinkCmd() *cobra.Command {
	var flagWorkspace string

	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link the current directory to a Workato workspace",
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

			projectRoot, err := config.FindProjectRoot(cwd)
			if err != nil {
				return fmt.Errorf("no wk.toml found. Run 'wk init' first.")
			}

			configPath := filepath.Join(projectRoot, config.ProjectFile)
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if flagWorkspace == "" {
				return fmt.Errorf("--workspace flag is required")
			}

			oldWorkspace := cfg.Workspace
			cfg.Workspace = flagWorkspace

			if err := config.Save(configPath, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			result := map[string]string{
				"status":         "linked",
				"workspace":      flagWorkspace,
				"prev_workspace": oldWorkspace,
				"path":           configPath,
			}

			if flagJSON {
				return rctx.Formatter.Format(cmd.OutOrStdout(), result)
			}

			if oldWorkspace != "" && oldWorkspace != flagWorkspace {
				fmt.Fprintf(cmd.OutOrStdout(), "Workspace updated from %q to %q in %s\n", oldWorkspace, flagWorkspace, configPath)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Linked workspace %q in %s\n", flagWorkspace, configPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Workspace profile to link")

	return cmd
}
