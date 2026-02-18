package commands

import (
	"fmt"
	"os"

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
	if flagProfile != "" {
		activeName = flagProfile
	}
	if activeName == "" {
		return nil, nil, fmt.Errorf("no active profile; run 'wk auth login' first or use --profile")
	}

	profile, err := pm.GetProfile(activeName)
	if err != nil {
		return nil, nil, fmt.Errorf("profile %q: %w", activeName, err)
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
