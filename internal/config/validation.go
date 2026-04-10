package config

import "fmt"

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
