package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
)

// HookParams is the input sent to a plugin's pre-push hook method.
type HookParams struct {
	ProjectRoot string     `json:"project_root"`
	Files       []HookFile `json:"files"`
}

// HookFile describes a single file in the hook payload.
type HookFile struct {
	Path       string `json:"path"`
	Status     string `json:"status"`                // "new", "modified", "deleted", "unchanged"
	ServerPath string `json:"server_path,omitempty"`
}

// HookResult is the response from a single plugin's hook execution.
type HookResult struct {
	PluginName  string       `json:"plugin_name"`
	Passed      bool         `json:"passed"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Diagnostic is a single issue reported by a hook.
type Diagnostic struct {
	File     string `json:"file"`
	Severity string `json:"severity"` // "error", "warning", "info"
	Message  string `json:"message"`
	Rule     string `json:"rule,omitempty"`
	Path     string `json:"path,omitempty"` // JSON pointer within file
}

// RunPrePushHook executes the pre-push hook for every installed plugin that
// declares one. Returns nil, nil if no plugins declare a pre-push hook.
// Individual plugin failures are fail-open: a warning HookResult is returned
// but the error does not propagate.
func RunPrePushHook(reg *Registry, params HookParams) ([]HookResult, error) {
	plugins, err := reg.List()
	if err != nil {
		return nil, fmt.Errorf("listing plugins: %w", err)
	}

	var results []HookResult
	for _, p := range plugins {
		m, err := LoadManifest(filepath.Join(p.Dir, "plugin.toml"))
		if err != nil || m.Hooks.PrePush == "" {
			continue
		}

		result, err := runSingleHook(p.Dir, m, params)
		if err != nil {
			// Fail-open: report as a warning result so the caller can print it.
			results = append(results, HookResult{
				PluginName: m.Name,
				Passed:     true,
				Diagnostics: []Diagnostic{{
					File:     "",
					Severity: "warning",
					Message:  fmt.Sprintf("pre-push hook failed: %v", err),
				}},
			})
			continue
		}
		result.PluginName = m.Name
		results = append(results, *result)
	}

	if len(results) == 0 {
		return nil, nil
	}
	return results, nil
}

// runSingleHook loads a plugin, calls its pre-push method, and returns the result.
func runSingleHook(pluginDir string, m *Manifest, params HookParams) (*HookResult, error) {
	host := NewPluginHost()
	defer host.StopAll()

	if err := host.Load(pluginDir); err != nil {
		return nil, fmt.Errorf("loading plugin: %w", err)
	}

	raw, err := host.Execute(m.Name, m.Hooks.PrePush, params)
	if err != nil {
		return nil, fmt.Errorf("executing hook: %w", err)
	}

	var result HookResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling hook result: %w", err)
	}
	return &result, nil
}

// FormatHookResults writes hook diagnostics to w.
// When asJSON is true, the results are marshaled as JSON.
// Otherwise, a human-readable text format is used.
func FormatHookResults(w io.Writer, results []HookResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	for _, r := range results {
		if len(r.Diagnostics) == 0 {
			continue
		}
		for _, d := range r.Diagnostics {
			prefix := d.Severity
			line := fmt.Sprintf("[%s]", prefix)
			if d.File != "" {
				line += " " + d.File
			}
			line += ": " + d.Message
			if d.Rule != "" {
				line += " (" + d.Rule + ")"
			}
			fmt.Fprintln(w, line)
		}
	}
	return nil
}
