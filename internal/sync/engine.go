package sync

import (
	"fmt"
	"os"

	"github.com/workato-devs/wk-cli-beta/internal/api"
	"github.com/workato-devs/wk-cli-beta/internal/config"
)

// SyncEngine coordinates pull, push, status, and diff operations
// between the local project directory and the Workato workspace.
type SyncEngine struct {
	projectRoot string
	config      *config.Config
	packages    api.PackageService
	folders     api.FolderService
}

// ignoreMatcher loads and returns the project's .wkignore matcher.
// A parse error is logged and a permissive (no-op) matcher is returned
// so that misconfiguration never blocks sync operations.
func (e *SyncEngine) ignoreMatcher() *Matcher {
	m, err := LoadMatcher(e.projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load %s: %v\n", IgnoreFile, err)
		return &Matcher{}
	}
	return m
}

// NewSyncEngine creates a SyncEngine wired to the given project and API client.
// client may be nil for local-only operations (e.g. status).
func NewSyncEngine(projectRoot string, cfg *config.Config, client api.Client) *SyncEngine {
	e := &SyncEngine{
		projectRoot: projectRoot,
		config:      cfg,
	}
	if client != nil {
		e.packages = client.Packages()
		e.folders = client.Folders()
	}
	return e
}

// projectRel returns path converted to a project-root-relative,
// forward-slash path suitable for feeding to the .wkignore matcher.
func (e *SyncEngine) projectRel(absPath string) (string, error) {
	return relSlashes(e.projectRoot, absPath)
}
