package sync

import (
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
