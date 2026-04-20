package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/workato-devs/wk-cli-beta/internal/config"
	wkerrors "github.com/workato-devs/wk-cli-beta/internal/errors"
)

// pollInterval is the delay between status checks when waiting for
// server-side export/import operations.
const pollInterval = 2 * time.Second

// implicitRootFolder is the Workato UI label for the workspace root.
// It does not correspond to an actual API folder.
const implicitRootFolder = "All projects"

// folderIDForEntry returns the Workato folder ID for entry. If the entry
// has a cached FolderID (non-zero), it is returned without any API calls.
// Otherwise the path is resolved via the folder-hierarchy walk and the ID
// is written back to wk.toml as a write-through cache (ADR-005 Decision 9).
func (e *SyncEngine) folderIDForEntry(ctx context.Context, entry config.SyncEntry) (int, error) {
	if entry.FolderID != 0 {
		return entry.FolderID, nil
	}
	return e.resolveAndCache(ctx, entry.ServerPath)
}

// resolveAndCache walks the folder hierarchy to resolve serverPath and
// persists the resulting ID to the matching [[sync]] entry in wk.toml.
// Cache write failures are logged but non-fatal — the ID is still returned.
func (e *SyncEngine) resolveAndCache(ctx context.Context, serverPath string) (int, error) {
	id, err := e.resolveFolderID(ctx, serverPath)
	if err != nil {
		return 0, err
	}
	if e.config != nil {
		for i := range e.config.Sync {
			if e.config.Sync[i].ServerPath == serverPath {
				e.config.Sync[i].FolderID = id
				break
			}
		}
	}
	if e.projectRoot != "" && e.config != nil {
		if err := config.Save(config.ProjectConfigPath(e.projectRoot), e.config); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not persist folder_id cache: %v\n", err)
		}
	}
	return id, nil
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

// resolveFolderID walks the Workato folder hierarchy to find the folder
// matching serverPath (e.g. "Recipes/Production/Integrations").
// The special name "All projects" is treated as the implicit workspace root
// and stripped from the path before resolution.
func (e *SyncEngine) resolveFolderID(ctx context.Context, serverPath string) (int, error) {
	parts := strings.Split(strings.Trim(serverPath, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("empty server path")
	}

	// Strip the implicit root folder if present.
	if strings.EqualFold(parts[0], implicitRootFolder) {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return 0, nil
	}

	var parentID *int
	for _, name := range parts {
		folders, err := e.listFolders(ctx, parentID)
		if err != nil {
			return 0, fmt.Errorf("listing folders under %v: %w", parentID, err)
		}
		found := false
		for _, f := range folders {
			if strings.EqualFold(f.Name, name) {
				id := f.ID
				parentID = &id
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("folder %q not found under parent %v: %w", name, parentID, errPathNotResolved)
		}
	}
	return *parentID, nil
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
