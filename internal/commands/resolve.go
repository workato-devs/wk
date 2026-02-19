package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/api"
	"github.com/workato-devs/wk-cli-beta/internal/auth"
	"github.com/workato-devs/wk-cli-beta/internal/config"
)

// resolveAPIClient builds an authenticated API client from the active profile
// or --profile flag override.
func resolveAPIClient(cmd *cobra.Command) (api.Client, *auth.Profile, error) {
	pm := auth.NewProfileManager()

	activeName, err := pm.GetActiveProfile()
	if err != nil {
		activeName = ""
	}
	explicitProfile := flagProfile != ""
	if explicitProfile {
		activeName = flagProfile
	}
	if activeName == "" {
		return nil, nil, fmt.Errorf("no active profile; run 'wk auth login' first or use --profile")
	}

	profile, err := pm.GetProfile(activeName)
	if err != nil {
		return nil, nil, fmt.Errorf("profile %q: %w", activeName, err)
	}

	// P0: Workspace isolation check — prevent accidental cross-workspace operations.
	// Only enforced when --profile was NOT explicitly set (explicit = intent override).
	if !explicitProfile {
		if cwd, err := os.Getwd(); err == nil {
			if projectRoot, err := config.FindProjectRoot(cwd); err == nil {
				if cfg, err := config.Load(filepath.Join(projectRoot, config.ProjectFile)); err == nil {
					if err := checkWorkspaceMatch(cfg, profile.Name); err != nil {
						return nil, nil, err
					}
				}
			}
		}
	}

	store := auth.NewChainStore(&auth.EnvStore{}, &auth.KeyringStore{})
	cred, err := store.Get(cmd.Context(), activeName)
	if err != nil {
		return nil, nil, fmt.Errorf("no credentials for profile %q: %w", activeName, err)
	}

	var opts []api.ClientOption
	if flagTimeout > 0 {
		opts = append(opts, api.WithTimeout(
			config.TimeoutDuration(flagTimeout),
		))
	}
	if flagVerbose {
		opts = append(opts, api.WithVerbose(true))
	}

	if flagVerbose {
		fmt.Fprintf(os.Stderr, "[debug] profile=%s region=%s base_url=%s\n", profile.Name, profile.Region, profile.BaseURL)
	}

	client := api.NewHTTPClient(profile.BaseURL+config.APIPathPrefix, cred.Token, opts...)
	return client, profile, nil
}

// checkWorkspaceMatch returns an error if the project config specifies a workspace
// that doesn't match the active profile name.
func checkWorkspaceMatch(cfg *config.Config, profileName string) error {
	if cfg.Workspace != "" && cfg.Workspace != profileName {
		return fmt.Errorf(
			"active profile %q does not match project workspace %q\n"+
				"Use --profile %s or run: wk auth switch %s",
			profileName, cfg.Workspace, cfg.Workspace, cfg.Workspace,
		)
	}
	return nil
}
