package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.toml")

	content := `name = "test-plugin"
version = "1.0.0"
description = "A test plugin"
entrypoint = "./bin/test"

[[commands]]
name = "greet"
description = "Say hello"
method = "test.greet"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Name != "test-plugin" {
		t.Errorf("Name = %q, want %q", m.Name, "test-plugin")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	if m.Entrypoint != "./bin/test" {
		t.Errorf("Entrypoint = %q, want %q", m.Entrypoint, "./bin/test")
	}
	if len(m.Commands) != 1 {
		t.Fatalf("Commands len = %d, want 1", len(m.Commands))
	}
	if m.Commands[0].Name != "greet" {
		t.Errorf("Commands[0].Name = %q, want %q", m.Commands[0].Name, "greet")
	}
	if m.Commands[0].Method != "test.greet" {
		t.Errorf("Commands[0].Method = %q, want %q", m.Commands[0].Method, "test.greet")
	}
}

func TestLoadManifestWithHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.toml")

	content := `name = "lint-plugin"
version = "0.2.0"
description = "A linter plugin"
entrypoint = "./bin/lint"

[hooks]
pre-push = "lint.pre_push"

[[commands]]
name = "lint"
description = "Run linter"
method = "lint.run"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Hooks.PrePush != "lint.pre_push" {
		t.Errorf("Hooks.PrePush = %q, want %q", m.Hooks.PrePush, "lint.pre_push")
	}
	if m.Hooks.PostPull != "" {
		t.Errorf("Hooks.PostPull = %q, want empty", m.Hooks.PostPull)
	}
}

func TestLoadManifestWithoutHooks(t *testing.T) {
	// Verify backward compat: existing manifests without [hooks] still parse fine.
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.toml")

	content := `name = "no-hooks"
version = "1.0.0"
description = "Plugin without hooks"
entrypoint = "./bin/test"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Hooks.PrePush != "" {
		t.Errorf("Hooks.PrePush = %q, want empty", m.Hooks.PrePush)
	}
}

func TestLoadManifestNotFound(t *testing.T) {
	_, err := LoadManifest("/nonexistent/plugin.toml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadManifestInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.toml")

	if err := os.WriteFile(path, []byte("this is not valid toml [[["), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}
