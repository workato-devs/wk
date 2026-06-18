package sync

import (
	"context"
	"fmt"
	"os"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/config"
)

// CreateMode controls folderIDForEntry's fallback behavior when the
// hierarchy walk fails to resolve an entry's server_path (ADR-007
// Decision 13). Default (zero value) is CreateModeBareNames.
type CreateMode int

const (
	// CreateModeBareNames creates a missing top-level folder when the
	// server_path has no slashes — the greenfield pattern. Nested paths
	// still error; auto-creating a multi-level tree on first push is
	// more likely to mask a typo than to match intent.
	CreateModeBareNames CreateMode = iota

	// CreateModeNever disables auto-create entirely. Any missing folder
	// surfaces as an error. The --no-create CI escape hatch.
	CreateModeNever

	// CreateModeAnyPath creates missing folders at any depth, walking
	// parents and creating each segment in order. The --create-path
	// escape hatch for deliberately spinning up a nested hierarchy.
	CreateModeAnyPath
)

// FolderCreated records one server-side folder creation triggered by
// push's resolve-then-create branch (ADR-007 Decision 14). One record
// per API call — under --create-path a single entry may produce
// multiple records (one per missing segment). ProjectID is non-zero
// when the API marked the created folder as a project (is_project=true);
// it is the separate identifier required by DELETE /projects/{project_id}.
type FolderCreated struct {
	ServerPath string `json:"server_path"`
	FolderID   int    `json:"folder_id"`
	ProjectID  int    `json:"project_id,omitempty"`
}

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

	// createMode controls folderIDForEntry's fallback when the walk
	// returns errPathNotResolved. Zero value = CreateModeBareNames.
	// Set by the push command after flag parsing.
	createMode CreateMode

	// foldersCreated accumulates server-folder creations from push's
	// resolve-then-create branch so the command layer can report them
	// loudly (ADR-007 Decision 14). Reset is caller-scoped — engines
	// are per-command-run, so a fresh one always starts empty.
	foldersCreated []FolderCreated
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

// SetCreateMode configures how folderIDForEntry handles a failed walk
// for uncached entries. Call once during command setup, after flag
// parsing. Default (zero value) is CreateModeBareNames.
func (e *SyncEngine) SetCreateMode(mode CreateMode) {
	e.createMode = mode
}

// FoldersCreated returns the server-folder creations that have
// happened during this engine's lifetime. Reset is caller-scoped: a
// fresh engine always starts empty. Safe to call once after all Push
// invocations complete.
//
// Always returns a non-nil slice so JSON consumers see "[]" rather
// than "null" when nothing was created — keeps scripts that destructure
// folders_created from having to special-case the empty case.
func (e *SyncEngine) FoldersCreated() []FolderCreated {
	if e.foldersCreated == nil {
		return []FolderCreated{}
	}
	return e.foldersCreated
}

// invalidateListCacheFor drops the cached folder list for parentID
// after a create under that parent. Subsequent resolves in this sweep
// will see the new folder. No-op when the cache is disabled.
func (e *SyncEngine) invalidateListCacheFor(parentID *int) {
	if e.listCache == nil {
		return
	}
	key := -1
	if parentID != nil {
		key = *parentID
	}
	delete(e.listCache, key)
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
