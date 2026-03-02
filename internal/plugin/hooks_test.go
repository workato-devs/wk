package plugin

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRunPrePushHookEmptyRegistry(t *testing.T) {
	dir := t.TempDir()
	reg := &Registry{Dir: dir}

	results, err := RunPrePushHook(reg, HookParams{ProjectRoot: "/tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results, got %v", results)
	}
}

func TestRunPrePushHookNoHooksDeclared(t *testing.T) {
	// Create a registry with a plugin that has no hooks.
	dir := t.TempDir()
	reg := &Registry{Dir: dir}

	pluginDir := dir + "/no-hooks"
	if err := writeTestManifest(pluginDir, `name = "no-hooks"
version = "1.0.0"
description = "no hooks"
entrypoint = "./bin/test"
`); err != nil {
		t.Fatal(err)
	}

	results, err := RunPrePushHook(reg, HookParams{ProjectRoot: "/tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results, got %v", results)
	}
}

func TestFormatHookResultsText(t *testing.T) {
	results := []HookResult{
		{
			PluginName: "recipe-lint",
			Passed:     false,
			Diagnostics: []Diagnostic{
				{File: "recipes/my_recipe.json", Severity: "error", Message: "missing required field", Rule: "required-fields"},
				{File: "recipes/other.json", Severity: "warning", Message: "deprecated connector"},
			},
		},
	}

	var buf bytes.Buffer
	if err := FormatHookResults(&buf, results, false); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "[error] recipes/my_recipe.json: missing required field (required-fields)") {
		t.Errorf("missing error diagnostic in output:\n%s", output)
	}
	if !strings.Contains(output, "[warning] recipes/other.json: deprecated connector") {
		t.Errorf("missing warning diagnostic in output:\n%s", output)
	}
}

func TestFormatHookResultsJSON(t *testing.T) {
	results := []HookResult{
		{
			PluginName:  "recipe-lint",
			Passed:      true,
			Diagnostics: []Diagnostic{},
		},
	}

	var buf bytes.Buffer
	if err := FormatHookResults(&buf, results, true); err != nil {
		t.Fatal(err)
	}

	var decoded []HookResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 result, got %d", len(decoded))
	}
	if decoded[0].PluginName != "recipe-lint" {
		t.Errorf("PluginName = %q, want %q", decoded[0].PluginName, "recipe-lint")
	}
	if !decoded[0].Passed {
		t.Error("expected Passed = true")
	}
}

func TestFormatHookResultsTextNoFile(t *testing.T) {
	// Diagnostic without a file (e.g., plugin execution warning).
	results := []HookResult{
		{
			PluginName: "broken-plugin",
			Passed:     true,
			Diagnostics: []Diagnostic{
				{Severity: "warning", Message: "pre-push hook failed: connection refused"},
			},
		},
	}

	var buf bytes.Buffer
	if err := FormatHookResults(&buf, results, false); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "[warning]: pre-push hook failed: connection refused") {
		t.Errorf("unexpected output:\n%s", output)
	}
}

// writeTestManifest writes a plugin.toml to a directory, creating the dir if needed.
func writeTestManifest(dir, content string) error {
	if err := mkdirAll(dir); err != nil {
		return err
	}
	return writeFile(dir+"/plugin.toml", content)
}

func mkdirAll(dir string) error {
	return mkdirAllPerm(dir, 0755)
}

func mkdirAllPerm(dir string, perm uint32) error {
	return os.MkdirAll(dir, os.FileMode(perm))
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
