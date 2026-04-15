package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wk.toml")

	cfg := &Config{
		Name:    "test-project",
		Profile: "dev",
		Sync: []SyncEntry{
			{ServerPath: "All projects/Test", LocalPath: "./recipes", Include: []string{"recipes"}},
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Name != cfg.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, cfg.Name)
	}
	if loaded.Profile != cfg.Profile {
		t.Errorf("Profile = %q, want %q", loaded.Profile, cfg.Profile)
	}
	if len(loaded.Sync) != 1 {
		t.Fatalf("Sync len = %d, want 1", len(loaded.Sync))
	}
	if loaded.Sync[0].ServerPath != "All projects/Test" {
		t.Errorf("Sync[0].ServerPath = %q, want %q", loaded.Sync[0].ServerPath, "All projects/Test")
	}
}

func TestFindProjectRoot(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	// No wk.toml — should fail
	_, err := FindProjectRoot(sub)
	if err == nil {
		t.Error("expected error when no wk.toml exists")
	}

	// Create wk.toml in the root dir
	cfg := &Config{Name: "test", Profile: "dev"}
	if err := Save(filepath.Join(dir, ProjectFile), cfg); err != nil {
		t.Fatal(err)
	}

	// Should find it from nested subdirectory
	root, err := FindProjectRoot(sub)
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{Name: "test", Profile: "dev"}, false},
		{"missing name", Config{Profile: "dev"}, true},
		{"missing profile", Config{Name: "test"}, true},
		{"bad sync entry", Config{Name: "test", Profile: "dev", Sync: []SyncEntry{{}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
