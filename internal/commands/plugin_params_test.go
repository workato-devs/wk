package commands

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/plugin"
)

func TestFlagToJSON(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"skills-path", "skills_path"},
		{"config-path", "config_path"},
		{"simple", "simple"},
		{"a-b-c", "a_b_c"},
	}
	for _, tt := range tests {
		if got := flagToJSON(tt.in); got != tt.want {
			t.Errorf("flagToJSON(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBuildPluginParams_NoDef(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	args := []string{"a.json", "b.json"}

	got := buildPluginParams(cmd, args, nil)
	arr, ok := got.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", got)
	}
	if !reflect.DeepEqual(arr, args) {
		t.Errorf("got %v, want %v", arr, args)
	}
}

func TestBuildPluginParams_EmptyDef(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	def := &pluginCmdDef{}

	got := buildPluginParams(cmd, []string{"x"}, def)
	if _, ok := got.([]string); !ok {
		t.Fatalf("expected []string for empty def, got %T", got)
	}
}

func TestBuildPluginParams_WithArgsAndFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "lint"}
	cmd.Flags().String("skills-path", "", "Path to skills")
	cmd.Flags().IntSlice("tiers", nil, "Lint tiers")

	// Simulate setting flags
	cmd.Flags().Set("skills-path", "/path/to/skills")
	cmd.Flags().Set("tiers", "1,2,3")

	def := &pluginCmdDef{
		Args: []plugin.Arg{
			{Name: "files", Required: true},
		},
		Flags: []plugin.Flag{
			{Name: "skills-path", Type: "string"},
			{Name: "tiers", Type: "int-array"},
		},
	}

	got := buildPluginParams(cmd, []string{"recipe.json"}, def)
	obj, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}

	// Check files
	files, ok := obj["files"].([]string)
	if !ok {
		t.Fatalf("files: expected []string, got %T", obj["files"])
	}
	if len(files) != 1 || files[0] != "recipe.json" {
		t.Errorf("files = %v, want [recipe.json]", files)
	}

	// Check skills_path
	if sp, ok := obj["skills_path"].(string); !ok || sp != "/path/to/skills" {
		t.Errorf("skills_path = %v, want /path/to/skills", obj["skills_path"])
	}

	// Check tiers
	tiers, ok := obj["tiers"].([]int)
	if !ok {
		t.Fatalf("tiers: expected []int, got %T", obj["tiers"])
	}
	if !reflect.DeepEqual(tiers, []int{1, 2, 3}) {
		t.Errorf("tiers = %v, want [1 2 3]", tiers)
	}
}

func TestBuildPluginParams_UnchangedFlagsOmitted(t *testing.T) {
	cmd := &cobra.Command{Use: "lint"}
	cmd.Flags().String("skills-path", "", "Path to skills")
	cmd.Flags().String("config-path", "", "Config path")

	// Only set one flag
	cmd.Flags().Set("skills-path", "/skills")

	def := &pluginCmdDef{
		Args: []plugin.Arg{{Name: "files"}},
		Flags: []plugin.Flag{
			{Name: "skills-path", Type: "string"},
			{Name: "config-path", Type: "string"},
		},
	}

	got := buildPluginParams(cmd, []string{}, def)
	obj := got.(map[string]any)

	if _, exists := obj["config_path"]; exists {
		t.Error("config_path should not be in params when flag not set")
	}
	if _, exists := obj["skills_path"]; !exists {
		t.Error("skills_path should be in params when flag is set")
	}
}

func TestBuildPluginParams_BoolFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("verbose", false, "Verbose output")
	cmd.Flags().Set("verbose", "true")

	def := &pluginCmdDef{
		Flags: []plugin.Flag{
			{Name: "verbose", Type: "bool"},
		},
	}

	got := buildPluginParams(cmd, nil, def)
	obj := got.(map[string]any)

	if v, ok := obj["verbose"].(bool); !ok || !v {
		t.Errorf("verbose = %v, want true", obj["verbose"])
	}
}
