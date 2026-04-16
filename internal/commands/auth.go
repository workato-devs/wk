package commands

import (
	"bufio"
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
	cmd.AddCommand(newAuthDeleteCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var (
		name        string
		workspace   string
		environment string
		region      string
		token       string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Create or update an auth profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)

			if flagJSON {
				// Non-interactive: all required flags must be provided.
				if name == "" {
					return fmt.Errorf("--name is required in non-interactive (--json) mode")
				}
				if workspace == "" {
					return fmt.Errorf("--workspace is required in non-interactive (--json) mode")
				}
				if environment == "" {
					return fmt.Errorf("--environment is required in non-interactive (--json) mode")
				}
				if token == "" {
					return fmt.Errorf("--token is required in non-interactive (--json) mode")
				}
			} else {
				// Interactive: prompt for missing values in struct field order.
				if name == "" {
					fmt.Print("Profile name: ")
					name, _ = reader.ReadString('\n')
					name = strings.TrimSpace(name)
				}
				if workspace == "" {
					fmt.Print("Workspace (Workato account name): ")
					workspace, _ = reader.ReadString('\n')
					workspace = strings.TrimSpace(workspace)
				}
				if environment == "" {
					fmt.Print("Environment (e.g. dev, staging, prod): ")
					environment, _ = reader.ReadString('\n')
					environment = strings.TrimSpace(environment)
				}
				if region == "" {
					fmt.Printf("Region [%s]: ", config.DefaultRegion)
					region, _ = reader.ReadString('\n')
					region = strings.TrimSpace(region)
					if region == "" {
						region = config.DefaultRegion
					}
				}
				if token == "" {
					fmt.Print("API token: ")
					token, _ = reader.ReadString('\n')
					token = strings.TrimSpace(token)
				}
			}

			if name == "" {
				return fmt.Errorf("profile name is required")
			}
			if workspace == "" {
				return fmt.Errorf("workspace is required")
			}
			if environment == "" {
				return fmt.Errorf("environment is required")
			}
			if token == "" {
				return fmt.Errorf("API token is required")
			}

			r := auth.Region(region)
			if !r.IsValid() {
				regions := auth.ValidRegions()
				names := make([]string, len(regions))
				for i, rg := range regions {
					names[i] = string(rg)
				}
				return fmt.Errorf("invalid region %q; valid regions: %s", region, strings.Join(names, ", "))
			}

			pm := auth.NewProfileManager()

			// Overwrite detection: check if profile name already exists.
			if existing, _ := pm.GetProfile(name); existing != nil && !force {
				if flagJSON {
					return fmt.Errorf("profile %q already exists — use --force to overwrite", name)
				}
				fmt.Fprintf(os.Stderr, "Profile %q already exists (workspace: %s, environment: %s, region: %s)\n",
					existing.Name, existing.Workspace, existing.Environment, existing.Region)
				fmt.Print("Overwrite? [y/N]: ")
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			baseURL := config.BaseURL(region)
			now := time.Now()

			profile := &auth.Profile{
				Name:        name,
				Workspace:   workspace,
				Environment: environment,
				Region:      r,
				StoreType:   auth.StoreKeychain,
				BaseURL:     baseURL,
				CreatedAt:   now,
			}

			cred := &auth.Credential{
				Token:     token,
				Region:    r,
				StoreType: auth.StoreKeychain,
				CreatedAt: now,
			}

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

			fmt.Fprintf(os.Stdout, "Profile %q saved and set as active (workspace: %s, environment: %s, region: %s)\n",
				name, workspace, environment, region)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Profile name")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workato account name")
	cmd.Flags().StringVar(&environment, "environment", "", "Target environment (e.g. dev, staging, prod)")
	cmd.Flags().StringVar(&region, "region", config.DefaultRegion, "Workato region (us, eu, jp, au, sg)")
	cmd.Flags().StringVar(&token, "token", "", "Workato API token")
	cmd.Flags().BoolVar(&force, "force", false, "Skip overwrite confirmation if profile already exists")
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
				Profile     string `json:"profile"`
				Workspace   string `json:"workspace"`
				Environment string `json:"environment"`
				Region      string `json:"region"`
				BaseURL     string `json:"base_url"`
				HasCreds    bool   `json:"has_credentials"`
				StoreType   string `json:"store_type"`
				Connected   bool   `json:"connected"`
				ConnError   string `json:"conn_error,omitempty"`
			}
			info := statusInfo{
				Profile:     profile.Name,
				Workspace:   profile.Workspace,
				Environment: profile.Environment,
				Region:      string(profile.Region),
				BaseURL:     profile.BaseURL,
				HasCreds:    credErr == nil,
				StoreType:   string(profile.StoreType),
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
				fmt.Fprintf(os.Stdout, "Workspace:   %s\n", info.Workspace)
				fmt.Fprintf(os.Stdout, "Environment: %s\n", info.Environment)
				fmt.Fprintf(os.Stdout, "Region:      %s\n", info.Region)
				fmt.Fprintf(os.Stdout, "Base URL:    %s\n", info.BaseURL)
				fmt.Fprintf(os.Stdout, "Credentials: %s\n", hasCreds)
				fmt.Fprintf(os.Stdout, "Store:       %s\n", info.StoreType)
				if info.Connected {
					fmt.Fprintf(os.Stdout, "API:         connected\n")
				} else if info.ConnError != "" {
					fmt.Fprintf(os.Stdout, "API:         %s\n", info.ConnError)
				}

				if profile.Workspace == "" || profile.Environment == "" {
					fmt.Fprintf(os.Stderr, "\nWarning: profile missing workspace/environment — run 'wk auth login' to update\n")
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

			headers := []string{"NAME", "WORKSPACE", "ENVIRONMENT", "REGION", "STORE", "ACTIVE"}
			var rows [][]string
			for _, p := range profiles {
				active := ""
				if p.Name == activeName {
					active = "*"
				}
				ws := p.Workspace
				if ws == "" {
					ws = "(unset)"
				}
				env := p.Environment
				if env == "" {
					env = "(unset)"
				}
				rows = append(rows, []string{p.Name, ws, env, string(p.Region), string(p.StoreType), active})
			}

			return rctx.Formatter.FormatList(os.Stdout, headers, rows)
		},
	}
}

func newAuthDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an auth profile and its stored credential",
		Args:  requireArgs(1, "profile name is required, e.g.: wk auth delete <name>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			pm := auth.NewProfileManager()

			profile, err := pm.GetProfile(name)
			if err != nil {
				return fmt.Errorf("profile %q not found", name)
			}

			// Remove credential from keyring.
			store := &auth.KeyringStore{}
			if err := store.Delete(cmd.Context(), name); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not remove credential from keyring: %v\n", err)
			}

			// Remove profile metadata.
			if err := pm.DeleteProfile(name); err != nil {
				return fmt.Errorf("deleting profile: %w", err)
			}

			// If this was the active profile, clear it.
			if activeName, _ := pm.GetActiveProfile(); activeName == name {
				_ = pm.SetActiveProfile("")
			}

			fmt.Fprintf(os.Stdout, "Deleted profile %q (workspace: %s, environment: %s)\n",
				profile.Name, profile.Workspace, profile.Environment)
			return nil
		},
	}
}
