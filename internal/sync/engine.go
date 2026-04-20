package sync

import (
	"context"
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

	// listCache memoizes GET /folders?parent_id=... responses for the
	// lifetime of the engine instance. nil means the cache is disabled
	// (normal pull/push/status — each call reaches the API freshly).
	// EnableFolderListCache turns it on for multi-entry sweeps like
	// `wk sync refresh`, where N entries would otherwise re-fetch the
	// same folder list N times. Keys are parent_id (-1 for the implicit
	// workspace root).
	listCache map[int][]api.Folder
}

// EnableFolderListCache turns on per-parent List memoization for this
// engine. Safe to call once before a batch of ClassifyEntry calls.
// Pull/push/status should NOT enable this — they can see mid-run server
// changes and must reach the API freshly.
func (e *SyncEngine) EnableFolderListCache() {
	if e.listCache == nil {
		e.listCache = make(map[int][]api.Folder)
	}
}

// listFolders is the cache-aware wrapper around e.folders.List. When the
// cache is disabled (nil), it delegates directly; otherwise it memoizes
// by parent_id and returns the cached slice on subsequent calls.
func (e *SyncEngine) listFolders(ctx context.Context, parentID *int) ([]api.Folder, error) {
	key := -1
	if parentID != nil {
		key = *parentID
	}
	if e.listCache != nil {
		if cached, ok := e.listCache[key]; ok {
			return cached, nil
		}
	}
	folders, err := e.folders.List(ctx, parentID)
	if err != nil {
		return nil, err
	}
	if e.listCache != nil {
		e.listCache[key] = folders
	}
	return folders, nil
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
