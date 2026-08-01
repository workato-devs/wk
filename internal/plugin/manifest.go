package plugin

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// HookConfig declares lifecycle hooks a plugin wants to intercept.
type HookConfig struct {
	PrePush  string `toml:"pre-push,omitempty"`
	PostPull string `toml:"post-pull,omitempty"` // reserved, not dispatched yet
}

// Manifest represents a plugin.toml file describing a plugin.
type Manifest struct {
	Name        string     `toml:"name"`
	Version     string     `toml:"version"`
	Description string     `toml:"description"`
	Entrypoint  string     `toml:"entrypoint"`
	Commands    []Command  `toml:"commands"`
	Hooks       HookConfig `toml:"hooks"`
}

// Command is a top-level command exposed by a plugin.
type Command struct {
	Name        string       `toml:"name"`
	Description string       `toml:"description"`
	Method      string       `toml:"method,omitempty"`
	Renderer    string       `toml:"renderer,omitempty"`
	Args        []Arg        `toml:"args,omitempty"`
	Flags       []Flag       `toml:"flags,omitempty"`
	Subcommands []Subcommand `toml:"subcommands,omitempty"`
}

// Subcommand is a nested command under a top-level plugin command.
type Subcommand struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Method      string `toml:"method"`
	Renderer    string `toml:"renderer,omitempty"`
	Args        []Arg  `toml:"args,omitempty"`
	Flags       []Flag `toml:"flags,omitempty"`
}

// RendererForMethod returns the optional text renderer declared for method.
// It is primarily used by built-in aliases that delegate to a plugin method
// without being registered from the plugin's command declaration.
func (m *Manifest) RendererForMethod(method string) string {
	for _, command := range m.Commands {
		if command.Method == method {
			return command.Renderer
		}
		for _, subcommand := range command.Subcommands {
			if subcommand.Method == method {
				return subcommand.Renderer
			}
		}
	}
	return ""
}

// Arg describes a positional argument for a command or subcommand.
type Arg struct {
	Name        string `toml:"name"`
	Description string `toml:"description,omitempty"`
	Required    bool   `toml:"required,omitempty"`
}

// Flag describes a named flag for a command or subcommand.
// Supported types: "string" (default), "int", "bool", "string-array", "int-array".
type Flag struct {
	Name        string `toml:"name"`
	Description string `toml:"description,omitempty"`
	Type        string `toml:"type,omitempty"`
	Default     string `toml:"default,omitempty"`
}

// LoadManifest reads and parses a plugin.toml file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
