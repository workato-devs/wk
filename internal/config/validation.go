package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateLocalPath enforces the container-relative boundary on a sync
// entry's local_path. Rejects empty strings, null bytes, absolute paths,
// and any form that (after cleaning) would resolve outside projectRoot.
// Applied to raw flag input (e.g. --projects-dir) and to every assembled
// SyncEntry.LocalPath. Pass an empty projectRoot to skip the on-disk
// resolution check and only run the string-level guards.
func ValidateLocalPath(projectRoot, localPath string) error {
	if localPath == "" {
		return fmt.Errorf("local path cannot be empty (use \".\" for the project root)")
	}
	if strings.ContainsRune(localPath, 0) {
		return fmt.Errorf("local path %q contains a null byte", localPath)
	}
	if filepath.IsAbs(localPath) {
		return fmt.Errorf("local path %q must be relative to the project root", localPath)
	}
	cleaned := filepath.Clean(localPath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("local path %q escapes the project root", localPath)
	}
	if projectRoot == "" {
		return nil
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolving project root: %w", err)
	}
	absLocal, err := filepath.Abs(filepath.Join(absRoot, cleaned))
	if err != nil {
		return fmt.Errorf("resolving local path: %w", err)
	}
	if absLocal != absRoot && !strings.HasPrefix(absLocal, absRoot+string(filepath.Separator)) {
		return fmt.Errorf("local path %q resolves outside the project root", localPath)
	}
	return nil
}

// Validate checks a Config for required fields and valid values.
func Validate(cfg *Config) error {
	if cfg.Name == "" {
		return fmt.Errorf("project name is required")
	}
	if cfg.Profile == "" {
		return fmt.Errorf("profile is required in wk.toml (run 'wk auth login' to create one)")
	}
	for i, s := range cfg.Sync {
		if s.ServerPath == "" {
			return fmt.Errorf("sync entry %d: server_path is required", i)
		}
		if s.LocalPath == "" {
			return fmt.Errorf("sync entry %d: local_path is required", i)
		}
	}
	return nil
}
