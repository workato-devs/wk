package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/workato-devs/wk/internal/telemetry"
)

func newTelemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage telemetry collection",
		Long: `Manage anonymous usage telemetry collection.

wk collects anonymous telemetry to understand how the CLI is used and prioritise
improvements. No credentials, file paths, project names, or flag values are collected.
See https://github.com/workato-devs/wk/blob/main/docs/adrs/ADR-010-telemetry.md
for the full data inventory.`,
	}
	cmd.AddCommand(newTelemetryStatusCmd())
	cmd.AddCommand(newTelemetryEnableCmd())
	cmd.AddCommand(newTelemetryDisableCmd())
	return cmd
}

func newTelemetryStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current telemetry status",
		RunE: func(cmd *cobra.Command, args []string) error {
			enabled := !telemetry.IsDisabled()
			byEnv := os.Getenv("DO_NOT_TRACK") == "1" || os.Getenv("WK_TELEMETRY_DISABLED") == "1"
			byCfg := telemetry.ReadConfig().Disabled

			if flagJSON {
				prefix := ""
				if enabled {
					prefix = telemetry.DeviceID()
					if len(prefix) > 8 {
						prefix = prefix[:8]
					}
				}
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"enabled":             enabled,
					"device_id_prefix":    prefix,
					"config_path":         telemetry.ConfigFilePath(),
					"disabled_by_env":     byEnv,
					"disabled_by_config":  byCfg,
				})
			}

			status := "enabled"
			if !enabled {
				status = "disabled"
			}
			fmt.Fprintf(os.Stdout, "Telemetry: %s\n", status)

			if enabled {
				prefix := telemetry.DeviceID()
				if len(prefix) > 8 {
					prefix = prefix[:8]
				}
				fmt.Fprintf(os.Stdout, "Device ID: %s...\n", prefix)
			}

			fmt.Fprintf(os.Stdout, "Config:    %s\n", telemetry.ConfigFilePath())
			fmt.Fprintln(os.Stdout, "Opt-out methods:")
			fmt.Fprintln(os.Stdout, "  WK_TELEMETRY_DISABLED=1   (environment variable)")
			fmt.Fprintln(os.Stdout, "  DO_NOT_TRACK=1            (environment variable — standard)")
			fmt.Fprintln(os.Stdout, "  wk telemetry disable      (persisted)")
			return nil
		},
	}
}

func newTelemetryEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable telemetry collection",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := telemetry.ReadConfig()
			cfg.Disabled = false
			if err := telemetry.WriteConfig(cfg); err != nil {
				return fmt.Errorf("updating telemetry config: %w", err)
			}
			if !flagQuiet {
				fmt.Fprintln(os.Stdout, "Telemetry enabled. Run `wk telemetry status` to verify.")
			}
			return nil
		},
	}
}

func newTelemetryDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable telemetry collection",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := telemetry.ReadConfig()
			cfg.Disabled = true
			if err := telemetry.WriteConfig(cfg); err != nil {
				return fmt.Errorf("updating telemetry config: %w", err)
			}
			if !flagQuiet {
				fmt.Fprintln(os.Stdout, "Telemetry disabled. No usage data will be collected.")
			}
			return nil
		},
	}
}
