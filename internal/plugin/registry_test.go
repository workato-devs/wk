package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	wkerrors "github.com/workato-devs/wk/internal/errors"
)

func TestRegistryInstallListRemove(t *testing.T) {
	// Create temp plugin source with manifest and dummy entrypoint
	srcDir := t.TempDir()
	manifest := `name = "test-plugin"
version = "1.0.0"
description = "Test"
entrypoint = "./test-plugin"

[[commands]]
name = "greet"
description = "Greet"
method = "test.greet"
`
	os.WriteFile(filepath.Join(srcDir, "plugin.toml"), []byte(manifest), 0644)
	os.WriteFile(filepath.Join(srcDir, "test-plugin"), []byte("#!/bin/sh\necho ok"), 0755)

	// Create temp registry dir
	regDir := t.TempDir()
	reg := &Registry{Dir: regDir}

	// Install
	if _, err := reg.Install(srcDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// List
	plugins, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "test-plugin" {
		t.Errorf("expected name 'test-plugin', got %q", plugins[0].Name)
	}

	// Remove
	if err := reg.Remove("test-plugin"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	plugins, _ = reg.List()
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins after remove, got %d", len(plugins))
	}
}

// writeSourceCheckout lays out a plugin source checkout: the runtime files
// plus repo contents that shouldn't ship (#101).
func writeSourceCheckout(t *testing.T, files string) string {
	t.Helper()
	src := t.TempDir()
	manifest := "name = \"p\"\nversion = \"1.0.0\"\nentrypoint = \"./p\"\n" + files
	for rel, body := range map[string]string{
		"plugin.toml":            manifest,
		"p":                      "#!/bin/sh\necho ok",
		"profiles/standard.json": "{}",
		"profiles/strict.json":   "{}",
		"profiles/embed.go":      "package profiles",
		"cmd/p/main.go":          "package main",
		"docs/README.md":         "internal docs",
		".git/config":            "[core]",
		"go.mod":                 "module p",
	} {
		path := filepath.Join(src, rel)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(body), 0755)
	}
	return src
}

// installedFiles lists every file under dir, relative and slash-separated.
func installedFiles(t *testing.T, dir string) []string {
	t.Helper()
	var got []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(got)
	return got
}

// Regression for #101: with `files` declared, install copies plugin.toml,
// the entrypoint and the declared files, not the rest of the source tree.
func TestRegistryInstall_DeclaredFiles(t *testing.T) {
	src := writeSourceCheckout(t, `files = ["profiles/*.json"]`+"\n")
	reg := &Registry{Dir: t.TempDir()}

	res, err := reg.Install(src)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.CopiedAll {
		t.Error("CopiedAll = true, want false when files is declared")
	}
	got := strings.Join(installedFiles(t, filepath.Join(reg.Dir, "p")), ",")
	want := "p,plugin.toml,profiles/standard.json,profiles/strict.json"
	if got != want {
		t.Errorf("installed files = %s, want %s", got, want)
	}
}

func TestRegistryInstall_DeclaredDirectory(t *testing.T) {
	src := writeSourceCheckout(t, `files = ["profiles"]`+"\n")
	reg := &Registry{Dir: t.TempDir()}

	if _, err := reg.Install(src); err != nil {
		t.Fatalf("Install: %v", err)
	}
	got := strings.Join(installedFiles(t, filepath.Join(reg.Dir, "p")), ",")
	want := "p,plugin.toml,profiles/embed.go,profiles/standard.json,profiles/strict.json"
	if got != want {
		t.Errorf("installed files = %s, want %s", got, want)
	}
}

// Without `files`, install keeps the old whole-directory copy and reports it,
// so existing plugins keep working.
func TestRegistryInstall_NoFilesCopiesAll(t *testing.T) {
	src := writeSourceCheckout(t, "")
	reg := &Registry{Dir: t.TempDir()}

	res, err := reg.Install(src)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !res.CopiedAll {
		t.Error("CopiedAll = false, want true when files is not declared")
	}
	if got := len(installedFiles(t, filepath.Join(reg.Dir, "p"))); got != 9 {
		t.Errorf("installed %d files, want all 9", got)
	}
}

func TestRegistryInstall_DeclaredFilesErrors(t *testing.T) {
	tests := []struct {
		name, files, entrypoint, wantErr string
	}{
		{"pattern matches nothing", `files = ["assets/*"]`, "./p", `"assets/*" matched nothing`},
		{"pattern escapes the plugin directory", `files = ["../*"]`, "./p", "reaches outside the plugin directory"},
		{"absolute pattern", `files = ["/etc/passwd"]`, "./p", "must be relative"},
		{"missing entrypoint", `files = ["profiles/*.json"]`, "./missing", `entrypoint "./missing" not found`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := writeSourceCheckout(t, tt.files+"\n")
			manifest, _ := os.ReadFile(filepath.Join(src, "plugin.toml"))
			os.WriteFile(filepath.Join(src, "plugin.toml"),
				[]byte(strings.Replace(string(manifest), `"./p"`, `"`+tt.entrypoint+`"`, 1)), 0644)

			// An existing install must survive a failed reinstall.
			reg := &Registry{Dir: t.TempDir()}
			os.MkdirAll(filepath.Join(reg.Dir, "p"), 0755)
			os.WriteFile(filepath.Join(reg.Dir, "p", "plugin.toml"), []byte(`name = "p"`), 0644)

			_, err := reg.Install(src)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
			if _, serr := os.Stat(filepath.Join(reg.Dir, "p", "plugin.toml")); serr != nil {
				t.Errorf("existing install was removed by a failed reinstall: %v", serr)
			}
		})
	}
}

// On Windows the manifest names the binary without its extension, and
// install must still find it to copy it.
func TestFindEntrypoint(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "tool"), []byte("x"), 0755)
	if got, err := findEntrypoint(src, "./tool"); err != nil || got != "tool" {
		t.Errorf("findEntrypoint(./tool) = %q, %v; want tool", got, err)
	}
	if runtime.GOOS == "windows" {
		os.WriteFile(filepath.Join(src, "lint.exe"), []byte("x"), 0755)
		if got, err := findEntrypoint(src, "./lint"); err != nil || got != "lint.exe" {
			t.Errorf("findEntrypoint(./lint) = %q, %v; want lint.exe", got, err)
		}
	}
}

func TestRegistryRemoveNotInstalled(t *testing.T) {
	reg := &Registry{Dir: t.TempDir()}
	err := reg.Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error removing non-existent plugin")
	}
	if !errors.Is(err, wkerrors.ErrPluginNotFound) {
		t.Errorf("expected errors.Is(err, ErrPluginNotFound), got: %v", err)
	}
	want := `plugin "nonexistent" is not installed`
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestRegistryGetPluginDirNotInstalled(t *testing.T) {
	reg := &Registry{Dir: t.TempDir()}
	_, err := reg.GetPluginDir("missing")
	if err == nil {
		t.Fatal("expected error for non-existent plugin")
	}
	if !errors.Is(err, wkerrors.ErrPluginNotFound) {
		t.Errorf("expected errors.Is(err, ErrPluginNotFound), got: %v", err)
	}
	want := `plugin "missing" is not installed`
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestRegistryListEmpty(t *testing.T) {
	reg := &Registry{Dir: t.TempDir()}
	plugins, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected 0, got %d", len(plugins))
	}
}
