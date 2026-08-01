package plugin

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/workato-devs/wk/internal/term"
)

// PluginHost manages multiple loaded plugin processes.
type PluginHost struct {
	plugins map[string]*loadedPlugin
	mu      sync.RWMutex
}

type loadedPlugin struct {
	Manifest *Manifest
	Client   *RPCClient
	Dir      string
}

// NewPluginHost creates an empty plugin host.
func NewPluginHost() *PluginHost {
	return &PluginHost{
		plugins: make(map[string]*loadedPlugin),
	}
}

// Load reads a plugin manifest from pluginDir and starts its RPC process.
func (h *PluginHost) Load(pluginDir string) error {
	manifestPath := filepath.Join(pluginDir, "plugin.toml")
	m, err := LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("loading manifest from %s: %w", pluginDir, err)
	}

	entrypoint := m.Entrypoint
	if !filepath.IsAbs(entrypoint) {
		entrypoint = filepath.Join(pluginDir, entrypoint)
	}

	client, err := NewRPCClient(entrypoint)
	if err != nil {
		return fmt.Errorf("starting plugin %s: %w", m.Name, err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// If a plugin with the same name is already loaded, stop it first.
	if existing, ok := h.plugins[m.Name]; ok {
		existing.Client.Close()
	}

	h.plugins[m.Name] = &loadedPlugin{
		Manifest: m,
		Client:   client,
		Dir:      pluginDir,
	}
	return nil
}

// Execute routes an RPC call to the named plugin.
func (h *PluginHost) Execute(pluginName, method string, params any) (json.RawMessage, error) {
	return h.execute(pluginName, method, params, true)
}

// Render calls a plugin's presentation-only renderer without displaying a
// second progress spinner for the same user command.
func (h *PluginHost) Render(pluginName, method string, params RenderRequest) (string, error) {
	result, err := h.execute(pluginName, method, params, false)
	if err != nil {
		return "", err
	}
	return DecodeRenderResponse(result)
}

func (h *PluginHost) execute(pluginName, method string, params any, showSpinner bool) (json.RawMessage, error) {
	h.mu.RLock()
	p, ok := h.plugins[pluginName]
	h.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("plugin %q is not loaded", pluginName)
	}

	if showSpinner {
		sp := term.NewSpinner(fmt.Sprintf("Running %s/%s", pluginName, method))
		sp.Start()
		defer sp.Stop()
	}

	return p.Client.Call(method, params)
}

// GetManifest returns the manifest for a loaded plugin, or nil if not found.
func (h *PluginHost) GetManifest(name string) *Manifest {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if p, ok := h.plugins[name]; ok {
		return p.Manifest
	}
	return nil
}

// ListLoaded returns the names of all loaded plugins.
func (h *PluginHost) ListLoaded() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	names := make([]string, 0, len(h.plugins))
	for name := range h.plugins {
		names = append(names, name)
	}
	return names
}

// StopAll stops all loaded plugin processes.
func (h *PluginHost) StopAll() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var firstErr error
	for name, p := range h.plugins {
		if err := p.Client.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("stopping plugin %s: %w", name, err)
		}
		delete(h.plugins, name)
	}
	return firstErr
}

// Stop stops a single loaded plugin process.
func (h *PluginHost) Stop(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	p, ok := h.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q is not loaded", name)
	}
	err := p.Client.Close()
	delete(h.plugins, name)
	return err
}
