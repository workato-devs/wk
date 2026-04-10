package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

func newLinkCmd() *cobra.Command {
	var flagLinkProfile string

	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link the current project to an auth profile",
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

			if flagLinkProfile == "" {
				return fmt.Errorf("--profile flag is required")
			}

			oldProfile := cfg.Profile
			cfg.Profile = flagLinkProfile

			if err := config.Save(configPath, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			result := map[string]string{
				"status":       "linked",
				"profile":      flagLinkProfile,
				"prev_profile": oldProfile,
				"path":         configPath,
			}

			if flagJSON {
				return rctx.Formatter.Format(cmd.OutOrStdout(), result)
			}

			if oldProfile != "" && oldProfile != flagLinkProfile {
				fmt.Fprintf(cmd.OutOrStdout(), "Profile updated from %q to %q in %s\n", oldProfile, flagLinkProfile, configPath)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Linked profile %q in %s\n", flagLinkProfile, configPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagLinkProfile, "profile", "", "Auth profile name to link")

	return cmd
}
