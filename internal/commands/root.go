package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk-cli-beta/internal/api"
	"github.com/workato-devs/wk-cli-beta/internal/auth"
	"github.com/workato-devs/wk-cli-beta/internal/config"
	"github.com/workato-devs/wk-cli-beta/internal/output"
	"github.com/workato-devs/wk-cli-beta/internal/plugin"
)

// Version info set by main via SetVersionInfo.
var (
	versionStr = "dev"
	commitStr  = "none"
	dateStr    = "unknown"
)

// SetVersionInfo is called from main.go to inject ldflags values.
func SetVersionInfo(version, commit, date string) {
	versionStr = version
	commitStr = commit
	dateStr = date
}

// RunContext carries resolved dependencies into every command handler.
// No global state — everything a command needs is here.
type RunContext struct {
	Config         *config.Config
	ProjectRoot    string
	AuthStore      auth.CredentialStore
	APIClient      api.Client
	Formatter      output.Formatter
	Profile        *auth.Profile
	PluginRegistry *plugin.Registry
	Verbose        bool
	Quiet          bool
}

var (
	flagJSON    bool
	flagVerbose bool
	flagQuiet   bool
	flagProfile string
	flagNoColor bool
	flagTimeout int
)

// NewRootCmd builds the root cobra command with all global flags.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "wk",
		Short: "Workato CLI - manage your Workato workspace from the terminal",
		Long: `wk is the official Workato CLI tool for managing recipes, connections,
sync operations, and plugins from your terminal or CI/CD pipeline.

Every command supports --json for machine-readable output.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := root.PersistentFlags()
	pf.BoolVar(&flagJSON, "json", false, "Output as JSON")
	pf.BoolVar(&flagVerbose, "verbose", false, "Enable verbose/debug logging")
	pf.BoolVar(&flagQuiet, "quiet", false, "Suppress non-essential output")
	pf.StringVar(&flagProfile, "profile", "", "Override active workspace profile")
	pf.BoolVar(&flagNoColor, "no-color", false, "Disable color output")
	pf.IntVar(&flagTimeout, "timeout", config.DefaultTimeout, "API timeout in seconds")

	return root
}

// BuildRunContext resolves dependencies for a command invocation.
func BuildRunContext(cmd *cobra.Command) (*RunContext, error) {
	format := "text"
	if flagJSON {
		format = "json"
	}

	rctx := &RunContext{
		Formatter: output.NewFormatter(format),
		Verbose:   flagVerbose,
		Quiet:     flagQuiet,
	}

	// Try to load project config (optional — not all commands need it)
	cwd, err := os.Getwd()
	if err == nil {
		if projectRoot, err := config.FindProjectRoot(cwd); err == nil {
			cfg, err := config.Load(filepath.Join(projectRoot, config.ProjectFile))
			if err == nil {
				rctx.Config = cfg
				rctx.ProjectRoot = projectRoot
			}
		}
	}

	// Initialize plugin registry (best-effort — not all environments have $HOME)
	if reg, err := plugin.NewRegistry(); err == nil {
		rctx.PluginRegistry = reg
	}

	return rctx, nil
}

// Execute runs the root command.
func Execute(ctx context.Context) int {
	root := NewRootCmd()
	registerAllCommands(root)

	if err := root.ExecuteContext(ctx); err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		return 1
	}
	return 0
}

// registerAllCommands wires all command groups into the root command.
// This is the single integration point — each command file provides a
// New*Cmd() function that is registered here.
func registerAllCommands(root *cobra.Command) {
	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newLinkCmd())
	root.AddCommand(newAuthCmd())
	root.AddCommand(newRecipesCmd())
	root.AddCommand(newConnectionsCmd())
	root.AddCommand(newPullCmd())
	root.AddCommand(newPushCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newDiffCmd())
	root.AddCommand(newCloneCmd())
	root.AddCommand(newPluginsCmd())
	root.AddCommand(newFoldersCmd())
	root.AddCommand(newTagsCmd())
	root.AddCommand(newAPICmd())
	root.AddCommand(newMCPCmd())
	root.AddCommand(newWorkspaceCmd())
	root.AddCommand(newConnectorsCmd())
	registerPluginCommands(root)
}

// hasCommand checks whether root already has a subcommand with the given name.
func hasCommand(root *cobra.Command, name string) bool {
	for _, c := range root.Commands() {
		if c.Name() == name {
			return true
		}
	}
	return false
}

// makePluginRunE creates a RunE function that loads a plugin and calls a method.
func makePluginRunE(pluginDir, method string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		host := plugin.NewPluginHost()
		defer host.StopAll()

		if err := host.Load(pluginDir); err != nil {
			return fmt.Errorf("loading plugin: %w", err)
		}

		m, _ := plugin.LoadManifest(filepath.Join(pluginDir, "plugin.toml"))
		if m == nil {
			return fmt.Errorf("cannot read plugin manifest")
		}

		result, err := host.Execute(m.Name, method, args)
		if err != nil {
			return err
		}

		if flagJSON {
			rctx, err := BuildRunContext(cmd)
			if err != nil {
				return err
			}
			return rctx.Formatter.Format(os.Stdout, json.RawMessage(result))
		}

		var m2 map[string]any
		if json.Unmarshal(result, &m2) == nil {
			for k, v := range m2 {
				fmt.Fprintf(os.Stdout, "%s: %v\n", k, v)
			}
			return nil
		}

		fmt.Fprintf(os.Stdout, "%s\n", string(result))
		return nil
	}
}

// registerPluginCommands discovers installed plugins and registers their
// commands on the root command.
func registerPluginCommands(root *cobra.Command) {
	registry, err := plugin.NewRegistry()
	if err != nil {
		return
	}

	plugins, err := registry.List()
	if err != nil || len(plugins) == 0 {
		return
	}

	for _, p := range plugins {
		m, err := plugin.LoadManifest(filepath.Join(p.Dir, "plugin.toml"))
		if err != nil {
			continue
		}

		for _, pcmd := range m.Commands {
			if hasCommand(root, pcmd.Name) {
				continue
			}

			if pcmd.Method != "" {
				root.AddCommand(&cobra.Command{
					Use:   pcmd.Name,
					Short: pcmd.Description,
					RunE:  makePluginRunE(p.Dir, pcmd.Method),
				})
			} else if len(pcmd.Subcommands) > 0 {
				parent := &cobra.Command{
					Use:   pcmd.Name,
					Short: pcmd.Description,
				}
				for _, sub := range pcmd.Subcommands {
					parent.AddCommand(&cobra.Command{
						Use:   sub.Name,
						Short: sub.Description,
						RunE:  makePluginRunE(p.Dir, sub.Method),
					})
				}
				root.AddCommand(parent)
			}
		}
	}
}
