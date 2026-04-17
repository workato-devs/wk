package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// ProjectFile is the name of the project configuration file.
const ProjectFile = "wk.toml"

// ProjectDir is the tool-managed directory at the project root that
// contains wk.toml and all sidecar metadata. Fully gitignored per
// ADR-005 Decision 8.
const ProjectDir = ".wk"

// ProjectConfigPath joins projectRoot with the canonical wk.toml location
// inside .wk/. Use this everywhere instead of filepath.Join(root, ProjectFile).
func ProjectConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ProjectDir, ProjectFile)
}

// Config represents the contents of a wk.toml project file.
//
// Workspace, WorkspaceID, Environment, and Email are an informational
// snapshot of the bound profile at init time (see ADR-006 Sub-decision 8).
// Runtime routing always resolves from the profile store; these fields
// exist so `cat wk.toml` reveals what the project targets at a glance.
type Config struct {
	Name        string      `toml:"name"`
	Description string      `toml:"description,omitempty"`
	Profile     string      `toml:"profile"`
	Workspace   string      `toml:"workspace,omitempty"`
	WorkspaceID int         `toml:"workspace_id,omitempty"`
	Environment string      `toml:"environment,omitempty"`
	Email       string      `toml:"email,omitempty"`
	Plugins     []string    `toml:"plugins,omitempty"`
	MCP         MCPConfig   `toml:"mcp,omitempty"`
	Sync        []SyncEntry `toml:"sync,omitempty"`
}

// MCPConfig holds MCP integration settings.
type MCPConfig struct {
	AutoDelegate bool   `toml:"auto_delegate"`
	ServerURL    string `toml:"server_url,omitempty"`
}

// SyncEntry maps a Workato server-side path to a local directory.
//
// FolderID caches the resolved Workato folder ID for ServerPath. When
// non-zero, sync operations skip the folder-hierarchy API walk. The
// sync engine populates it on first resolution (write-through cache)
// and invalidates + re-resolves on API 404. See ADR-005 Decision 9.
type SyncEntry struct {
	ServerPath string   `toml:"server_path"`
	LocalPath  string   `toml:"local_path"`
	FolderID   int      `toml:"folder_id,omitempty"`
	Include    []string `toml:"include,omitempty"`
}

// Load reads and parses a wk.toml file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes a Config to a wk.toml file at the given path.
func Save(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// FindProjectRoot walks up from startDir looking for a .wk/wk.toml file.
// Returns the directory containing .wk/ (the project root, not .wk/ itself),
// or an error if none is found. See ADR-005 Decision 5.
func FindProjectRoot(startDir string) (string, error) {
	dir := startDir
	for {
		configPath := filepath.Join(dir, ProjectDir, ProjectFile)
		if _, err := os.Stat(configPath); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s/%s found in %s or any parent directory", ProjectDir, ProjectFile, startDir)
		}
		dir = parent
	}
}
