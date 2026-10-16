package plugin

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	wkerrors "github.com/workato-devs/wk/internal/errors"
)

// Registry manages installed plugins on disk at ~/.wk/plugins/.
type Registry struct {
	Dir string
}

// InstalledPlugin describes a plugin found in the registry.
type InstalledPlugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Dir     string `json:"dir"`
}

// NewRegistry creates a Registry with the default directory (~/.wk/plugins/).
// It creates the directory if it does not exist.
// If WK_HOME is set, it is used instead of ~/.wk/ (useful for testing).
func NewRegistry() (*Registry, error) {
	base := os.Getenv("WK_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("determining home directory: %w", err)
		}
		base = filepath.Join(home, ".wk")
	}
	dir := filepath.Join(base, "plugins")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating plugins directory: %w", err)
	}
	return &Registry{Dir: dir}, nil
}

// InstallResult reports how a plugin was installed.
type InstallResult struct {
	Name string
	// CopiedAll is true when the manifest declares no `files`, so the whole
	// source directory was copied. Callers should warn: from a source
	// checkout that includes everything in the repo (#101).
	CopiedAll bool
}

// Install copies a plugin from source into the registry. It reads the
// plugin.toml at source to determine the plugin name. When the manifest
// declares `files`, only plugin.toml, the entrypoint, and those files are
// copied; otherwise the whole source directory is.
func (r *Registry) Install(source string) (*InstallResult, error) {
	manifestPath := filepath.Join(source, "plugin.toml")
	m, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading plugin manifest: %w", err)
	}

	// Resolve everything to copy before touching an existing installation,
	// so a bad manifest can't leave the plugin uninstalled.
	var paths []string
	if len(m.Files) > 0 {
		if paths, err = declaredPaths(source, m); err != nil {
			return nil, err
		}
	}

	dest := filepath.Join(r.Dir, m.Name)

	// Remove existing installation if present
	if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return nil, fmt.Errorf("removing existing plugin: %w", err)
		}
	}

	if paths == nil {
		if err := copyDir(source, dest); err != nil {
			return nil, fmt.Errorf("installing plugin: %w", err)
		}
		return &InstallResult{Name: m.Name, CopiedAll: true}, nil
	}
	for _, rel := range paths {
		if err := copyDir(filepath.Join(source, rel), filepath.Join(dest, rel)); err != nil {
			return nil, fmt.Errorf("installing plugin: %w", err)
		}
	}
	return &InstallResult{Name: m.Name}, nil
}

// declaredPaths returns the source-relative paths an install with declared
// `files` copies: plugin.toml, the entrypoint (unless absolute), and every
// match of each pattern. The author stated what ships, so a pattern that
// matches nothing, a path outside source, or a missing entrypoint is an error.
func declaredPaths(source string, m *Manifest) ([]string, error) {
	paths := []string{"plugin.toml"}

	if m.Entrypoint != "" && !filepath.IsAbs(m.Entrypoint) {
		entry, err := findEntrypoint(source, m.Entrypoint)
		if err != nil {
			return nil, err
		}
		paths = append(paths, entry)
	}

	for _, pattern := range m.Files {
		if filepath.IsAbs(pattern) {
			return nil, fmt.Errorf("plugin.toml files entry %q must be relative to the plugin directory", pattern)
		}
		matches, err := filepath.Glob(filepath.Join(source, pattern))
		if err != nil {
			return nil, fmt.Errorf("plugin.toml files entry %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("plugin.toml files entry %q matched nothing in %s", pattern, source)
		}
		for _, match := range matches {
			rel, err := filepath.Rel(source, match)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("plugin.toml files entry %q reaches outside the plugin directory", pattern)
			}
			paths = append(paths, rel)
		}
	}
	return paths, nil
}

// findEntrypoint returns the source-relative path of the entrypoint file.
// On Windows a manifest usually names the binary without its extension
// ("./recipe-lint" for recipe-lint.exe) and still runs, because os/exec tries
// the executable extensions there; install has to find the same file.
func findEntrypoint(source, entrypoint string) (string, error) {
	candidates := []string{entrypoint}
	if runtime.GOOS == "windows" && filepath.Ext(entrypoint) == "" {
		for _, ext := range []string{".exe", ".cmd", ".bat"} {
			candidates = append(candidates, entrypoint+ext)
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(source, c)); err == nil {
			return filepath.Clean(c), nil
		}
	}
	return "", fmt.Errorf("plugin entrypoint %q not found in %s", entrypoint, source)
}

// List scans the registry directory for installed plugins.
func (r *Registry) List() ([]InstalledPlugin, error) {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		return nil, fmt.Errorf("reading plugins directory: %w", err)
	}

	var plugins []InstalledPlugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(r.Dir, entry.Name(), "plugin.toml")
		m, err := LoadManifest(manifestPath)
		if err != nil {
			continue // skip directories without valid manifests
		}
		plugins = append(plugins, InstalledPlugin{
			Name:    m.Name,
			Version: m.Version,
			Dir:     filepath.Join(r.Dir, entry.Name()),
		})
	}
	return plugins, nil
}

// Remove deletes an installed plugin by name.
func (r *Registry) Remove(name string) error {
	dir := filepath.Join(r.Dir, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return wkerrors.WithSentinel(wkerrors.ErrPluginNotFound, "plugin %q is not installed", name)
	}
	return os.RemoveAll(dir)
}

// GetPluginDir returns the path to an installed plugin's directory.
func (r *Registry) GetPluginDir(name string) (string, error) {
	dir := filepath.Join(r.Dir, name)
	if _, err := os.Stat(filepath.Join(dir, "plugin.toml")); err != nil {
		return "", wkerrors.WithSentinel(wkerrors.ErrPluginNotFound, "plugin %q is not installed", name)
	}
	return dir, nil
}

// copyDir recursively copies src to dst. src may also be a single file.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
