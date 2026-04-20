package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/workato-devs/wk-cli-beta/internal/api"
	"github.com/workato-devs/wk-cli-beta/internal/config"
	wkerrors "github.com/workato-devs/wk-cli-beta/internal/errors"
)

// pollInterval is the delay between status checks when waiting for
// server-side export/import operations.
const pollInterval = 2 * time.Second

// implicitRootFolder is the Workato UI label for the workspace root.
// It does not correspond to an actual API folder.
const implicitRootFolder = "All projects"

// folderIDForEntry returns the Workato folder ID for entry. If the
// entry has a cached FolderID (non-zero), it is returned without any
// API calls. Otherwise the path is resolved via the folder-hierarchy
// walk and the ID is written back to wk.toml as a write-through cache
// (ADR-005 Decision 9). When the walk fails with errPathNotResolved,
// push's resolve-then-create branch (ADR-007 Decision 12) kicks in
// per the engine's createMode.
func (e *SyncEngine) folderIDForEntry(ctx context.Context, entry config.SyncEntry) (int, error) {
	if entry.FolderID != 0 {
		return entry.FolderID, nil
	}
	id, err := e.resolveAndCache(ctx, entry.ServerPath)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, errPathNotResolved) {
		return 0, err
	}
	return e.createForEntry(ctx, entry.ServerPath)
}

// createForEntry implements the resolve-then-create fallback for
// push (ADR-007 Decisions 12–13). Gating:
//   - CreateModeNever         — error on any missing segment.
//   - CreateModeBareNames     — create only if server_path is a bare
//                               name (no slashes); nested missing paths
//                               error.
//   - CreateModeAnyPath       — walk the hierarchy, creating each
//                               missing segment in order.
//
// Every API create is recorded on e.foldersCreated so the command
// layer can surface the events. The cached list for each parent is
// invalidated after a create so subsequent resolves in the same sweep
// see the new folder.
func (e *SyncEngine) createForEntry(ctx context.Context, serverPath string) (int, error) {
	if e.createMode == CreateModeNever {
		return 0, fmt.Errorf("folder %q not found and --no-create was set", serverPath)
	}

	parts := strings.Split(strings.Trim(serverPath, "/"), "/")
	if len(parts) > 0 && strings.EqualFold(parts[0], implicitRootFolder) {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return 0, fmt.Errorf("cannot create the implicit workspace root")
	}

	if len(parts) > 1 && e.createMode != CreateModeAnyPath {
		return 0, fmt.Errorf("nested path %q does not resolve; pass --create-path to create the full hierarchy", serverPath)
	}

	var parentID *int
	var leafFolderID, leafProjectID int
	for i, name := range parts {
		folders, err := e.listFolders(ctx, parentID)
		if err != nil {
			return 0, fmt.Errorf("listing folders under %v: %w", parentID, err)
		}
		found := false
		for _, f := range folders {
			if strings.EqualFold(f.Name, name) {
				leafFolderID = f.ID
				leafProjectID = f.ProjectID
				found = true
				break
			}
		}
		if !found {
			created, err := e.folders.Create(ctx, name, parentID)
			if err != nil {
				return 0, fmt.Errorf("creating folder %q: %w", name, err)
			}
			leafFolderID = created.ID
			leafProjectID = created.ProjectID
			e.foldersCreated = append(e.foldersCreated, FolderCreated{
				ServerPath: strings.Join(parts[:i+1], "/"),
				FolderID:   leafFolderID,
				ProjectID:  leafProjectID,
			})
			e.invalidateListCacheFor(parentID)
		}
		idCopy := leafFolderID
		parentID = &idCopy
	}

	// Write-through cache for the leaf IDs, mirroring resolveAndCache.
	e.writeCachedIDs(serverPath, leafFolderID, leafProjectID)
	if e.projectRoot != "" && e.config != nil {
		if err := config.Save(config.ProjectConfigPath(e.projectRoot), e.config); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not persist folder_id cache: %v\n", err)
		}
	}
	return leafFolderID, nil
}

// resolveAndCache walks the folder hierarchy to resolve serverPath and
// persists both folder_id AND project_id to the matching [[sync]]
// entry in wk.toml. project_id is zero for plain folders and populated
// for top-level projects (used by DELETE /projects/{project_id}).
// Cache write failures are logged but non-fatal — the ID is still returned.
func (e *SyncEngine) resolveAndCache(ctx context.Context, serverPath string) (int, error) {
	f, err := e.resolveFolder(ctx, serverPath)
	if err != nil {
		return 0, err
	}
	if f == nil {
		return 0, nil
	}
	e.writeCachedIDs(serverPath, f.ID, f.ProjectID)
	if e.projectRoot != "" && e.config != nil {
		if err := config.Save(config.ProjectConfigPath(e.projectRoot), e.config); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not persist folder_id cache: %v\n", err)
		}
	}
	return f.ID, nil
}

// writeCachedIDs updates both folder_id and project_id on the matching
// [[sync]] entry in memory. Callers batch config.Save once per sweep.
// projectID is set verbatim — zero for plain folders, non-zero for
// projects — so stale project_id values do get cleared if the server
// folder stopped being a project between syncs.
func (e *SyncEngine) writeCachedIDs(serverPath string, folderID, projectID int) {
	if e.config == nil {
		return
	}
	for i := range e.config.Sync {
		if e.config.Sync[i].ServerPath == serverPath {
			e.config.Sync[i].FolderID = folderID
			e.config.Sync[i].ProjectID = projectID
			return
		}
	}
}

// invalidFolderCacheErr reports whether err indicates that a cached folder_id
// is stale (the API no longer knows the folder). ADR-005 Decision 9 says to
// fall back to path resolution and retry in this case.
func invalidFolderCacheErr(err error) bool {
	return errors.Is(err, wkerrors.ErrAPINotFound)
}

// errPathNotResolved distinguishes "every List call succeeded but the
// requested name was not present at the expected parent" from
// API-level errors (auth, 5xx, network). Callers use errors.Is to
// decide whether to classify an entry as not-found/stale vs. to
// propagate the failure and halt a sweep.
var errPathNotResolved = errors.New("folder path not resolved")

// wrapFolderErr annotates a folder-resolution or API error with the sync
// entry's server_path, cached folder_id, and (when available) the snapshot
// workspace name, so developers can tell which [[sync]] entry failed and
// whether a workspace-mismatch is likely the cause.
func (e *SyncEngine) wrapFolderErr(err error, entry config.SyncEntry, origFolderID int) error {
	if err == nil {
		return nil
	}
	workspace := ""
	if e.config != nil && e.config.Workspace != "" {
		workspace = e.config.Workspace
	}
	if errors.Is(err, wkerrors.ErrAPINotFound) && origFolderID != 0 {
		if workspace != "" {
			return fmt.Errorf("folder_id %d (server_path %q) not found in workspace %q — cached value may be from a different workspace: %w", origFolderID, entry.ServerPath, workspace, err)
		}
		return fmt.Errorf("folder_id %d (server_path %q) not found — cached value may be from a different workspace: %w", origFolderID, entry.ServerPath, err)
	}
	if workspace != "" {
		return fmt.Errorf("resolving folder %q in workspace %q: %w", entry.ServerPath, workspace, err)
	}
	return fmt.Errorf("resolving folder %q: %w", entry.ServerPath, err)
}

// resolveFolder walks the Workato folder hierarchy to find the folder
// matching serverPath (e.g. "Recipes/Production/Integrations") and
// returns the full leaf Folder. The "All projects" prefix is treated
// as the implicit workspace root and stripped before resolution; when
// the path resolves to the root itself, returns (nil, nil).
//
// Callers that only need the folder_id wrap this via resolveFolderID;
// those that need project_id (e.g. wk.toml cache writes) take the
// full Folder.
func (e *SyncEngine) resolveFolder(ctx context.Context, serverPath string) (*api.Folder, error) {
	parts := strings.Split(strings.Trim(serverPath, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("empty server path")
	}

	if strings.EqualFold(parts[0], implicitRootFolder) {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return nil, nil
	}

	var parentID *int
	var leaf api.Folder
	for _, name := range parts {
		folders, err := e.listFolders(ctx, parentID)
		if err != nil {
			return nil, fmt.Errorf("listing folders under %v: %w", parentID, err)
		}
		found := false
		for _, f := range folders {
			if strings.EqualFold(f.Name, name) {
				leaf = f
				id := f.ID
				parentID = &id
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("folder %q not found under parent %v: %w", name, parentID, errPathNotResolved)
		}
	}
	return &leaf, nil
}

// resolveFolderID is the int-only wrapper around resolveFolder for
// callers that don't need the full folder metadata.
func (e *SyncEngine) resolveFolderID(ctx context.Context, serverPath string) (int, error) {
	f, err := e.resolveFolder(ctx, serverPath)
	if err != nil {
		return 0, err
	}
	if f == nil {
		return 0, nil
	}
	return f.ID, nil
}

// waitForPackage polls the export status until the package is complete.
func (e *SyncEngine) waitForPackage(ctx context.Context, pkgID int) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for export (package %d): %w", pkgID, ctx.Err())
		default:
		}

		pkg, err := e.packages.ExportStatus(ctx, pkgID)
		if err != nil {
			return fmt.Errorf("checking export status: %w", err)
		}

		switch pkg.Status {
		case "completed", "succeeded":
			return nil
		case "failed", "error":
			msg := fmt.Sprintf("export failed (package %d): status %s", pkgID, pkg.Status)
			if pkg.Error != "" {
				msg += ": " + pkg.Error
			}
			if len(pkg.ErrorParts) > 0 {
				msg += fmt.Sprintf(" (details: %v)", pkg.ErrorParts)
			}
			return fmt.Errorf("%s", msg)
		}

		time.Sleep(pollInterval)
	}
}

// waitForImport polls the import status until the import is complete.
func (e *SyncEngine) waitForImport(ctx context.Context, importID int) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for import %d: %w", importID, ctx.Err())
		default:
		}

		pkg, err := e.packages.ImportStatus(ctx, importID)
		if err != nil {
			return fmt.Errorf("checking import status: %w", err)
		}

		switch pkg.Status {
		case "completed", "succeeded":
			return nil
		case "failed", "error":
			msg := fmt.Sprintf("import failed (import %d): status %s", importID, pkg.Status)
			if pkg.Error != "" {
				msg += ": " + pkg.Error
			}
			if len(pkg.ErrorParts) > 0 {
				msg += fmt.Sprintf(" (details: %v)", pkg.ErrorParts)
			}
			return fmt.Errorf("%s", msg)
		}

		time.Sleep(pollInterval)
	}
}

// readFileBytes reads a file and returns its contents.
func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// relSlashes returns the path of target relative to base as a
// forward-slash-separated string, regardless of OS separator.
func relSlashes(base, target string) (string, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// findLocalFiles walks a directory and returns all file paths relative
// to that directory, skipping the .wk/ tool directory if encountered.
func findLocalFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".wk" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	return files, err
}
