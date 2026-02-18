package plugin

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Manifest represents a plugin.toml file describing a plugin.
type Manifest struct {
	Name        string    `toml:"name"`
	Version     string    `toml:"version"`
	Description string    `toml:"description"`
	Entrypoint  string    `toml:"entrypoint"`
	Commands    []Command `toml:"commands"`
}

// Command is a top-level command exposed by a plugin.
type Command struct {
	Name        string       `toml:"name"`
	Description string       `toml:"description"`
	Method      string       `toml:"method,omitempty"`
	Subcommands []Subcommand `toml:"subcommands,omitempty"`
}

// Subcommand is a nested command under a top-level plugin command.
type Subcommand struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Method      string `toml:"method"`
	Args        []Arg  `toml:"args,omitempty"`
}

// Arg describes a positional argument for a subcommand.
type Arg struct {
	Name        string `toml:"name"`
	Description string `toml:"description,omitempty"`
	Required    bool   `toml:"required,omitempty"`
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
