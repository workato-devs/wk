package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/api"
	"github.com/workato-devs/wk-cli-beta/internal/auth"
	"github.com/workato-devs/wk-cli-beta/internal/config"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication profiles",
	}
	cmd.AddCommand(newAuthLoginCmd())
	cmd.AddCommand(newAuthStatusCmd())
	cmd.AddCommand(newAuthSwitchCmd())
	cmd.AddCommand(newAuthListCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var token, region, name string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Create or update an auth profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				return fmt.Errorf("--token is required")
			}
			if name == "" {
				name = "default"
			}
			r := auth.Region(region)
			if !r.IsValid() {
				regions := auth.ValidRegions()
				names := make([]string, len(regions))
				for i, r := range regions {
					names[i] = string(r)
				}
				return fmt.Errorf("invalid region %q; valid regions: %s", region, strings.Join(names, ", "))
			}

			baseURL := config.BaseURL(region)
			now := time.Now()

			profile := &auth.Profile{
				Name:      name,
				Region:    r,
				StoreType: auth.StoreKeychain,
				BaseURL:   baseURL,
				CreatedAt: now,
			}

			cred := &auth.Credential{
				Token:     token,
				Region:    r,
				StoreType: auth.StoreKeychain,
				CreatedAt: now,
			}

			pm := auth.NewProfileManager()
			if err := pm.SaveProfile(profile); err != nil {
				return fmt.Errorf("saving profile: %w", err)
			}

			store := &auth.KeyringStore{}
			if err := store.Set(cmd.Context(), name, cred); err != nil {
				return fmt.Errorf("storing credential: %w", err)
			}

			if err := pm.SetActiveProfile(name); err != nil {
				return fmt.Errorf("setting active profile: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Profile %q saved and set as active (region: %s)\n", name, region)
			return nil
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "Workato API token (required)")
	cmd.Flags().StringVar(&region, "region", config.DefaultRegion, "Workato region (us, eu, jp, au, sg)")
	cmd.Flags().StringVar(&name, "name", "default", "Profile name")
	return cmd
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show active profile and test connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}

			pm := auth.NewProfileManager()
			activeName, err := pm.GetActiveProfile()
			if err != nil {
				return fmt.Errorf("no active profile: %w", err)
			}

			profile, err := pm.GetProfile(activeName)
			if err != nil {
				return fmt.Errorf("profile %q: %w", activeName, err)
			}

			store := auth.NewChainStore(&auth.EnvStore{}, &auth.KeyringStore{})
			_, credErr := store.Get(cmd.Context(), activeName)

			type statusInfo struct {
				Profile   string `json:"profile"`
				Region    string `json:"region"`
				BaseURL   string `json:"base_url"`
				HasCreds  bool   `json:"has_credentials"`
				StoreType string `json:"store_type"`
				Connected bool   `json:"connected"`
				ConnError string `json:"conn_error,omitempty"`
			}
			info := statusInfo{
				Profile:   profile.Name,
				Region:    string(profile.Region),
				BaseURL:   profile.BaseURL,
				HasCreds:  credErr == nil,
				StoreType: string(profile.StoreType),
			}

			if credErr == nil {
				client, _, clientErr := resolveAPIClient(cmd)
				if clientErr == nil {
					ctx := cmd.Context()
					_, apiErr := client.Recipes().List(ctx, &api.RecipeListOptions{PerPage: 1})
					if apiErr == nil {
						info.Connected = true
					} else {
						info.ConnError = apiErr.Error()
					}
				} else {
					info.ConnError = clientErr.Error()
				}
			}

			if !flagJSON {
				hasCreds := "no"
				if info.HasCreds {
					hasCreds = "yes"
				}
				fmt.Fprintf(os.Stdout, "Profile:     %s\n", info.Profile)
				fmt.Fprintf(os.Stdout, "Region:      %s\n", info.Region)
				fmt.Fprintf(os.Stdout, "Base URL:    %s\n", info.BaseURL)
				fmt.Fprintf(os.Stdout, "Credentials: %s\n", hasCreds)
				fmt.Fprintf(os.Stdout, "Store:       %s\n", info.StoreType)
				if info.Connected {
					fmt.Fprintf(os.Stdout, "API:         connected\n")
				} else if info.ConnError != "" {
					fmt.Fprintf(os.Stdout, "API:         %s\n", info.ConnError)
				}
				return nil
			}
			return rctx.Formatter.Format(os.Stdout, info)
		},
	}
}

func newAuthSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <name>",
		Short: "Switch active profile",
		Args:  requireArgs(1, "profile name is required, e.g.: wk auth switch <name>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			pm := auth.NewProfileManager()

			if _, err := pm.GetProfile(name); err != nil {
				return fmt.Errorf("profile %q not found", name)
			}

			if err := pm.SetActiveProfile(name); err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "Switched to profile %q\n", name)
			return nil
		},
	}
}

func newAuthListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all auth profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}

			pm := auth.NewProfileManager()
			profiles, err := pm.ListProfiles()
			if err != nil {
				return err
			}

			activeName, _ := pm.GetActiveProfile()

			headers := []string{"NAME", "REGION", "STORE", "ACTIVE"}
			var rows [][]string
			for _, p := range profiles {
				active := ""
				if p.Name == activeName {
					active = "*"
				}
				rows = append(rows, []string{p.Name, string(p.Region), string(p.StoreType), active})
			}

			return rctx.Formatter.FormatList(os.Stdout, headers, rows)
		},
	}
}
