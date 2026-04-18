package sync

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/workato-devs/wk-cli-beta/internal/config"
	wkerrors "github.com/workato-devs/wk-cli-beta/internal/errors"
)

// PullResult describes what happened to a single file during pull.
type PullResult struct {
	FilePath string `json:"file_path"`
	Action   string `json:"action"` // "created", "updated", "unchanged", "skipped"
}

// Pull downloads remote assets to the local project directory.
// If force is false, it checks for local modifications and aborts on conflicts.
func (e *SyncEngine) Pull(entry config.SyncEntry, force bool) ([]PullResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	localDir := filepath.Join(e.projectRoot, entry.LocalPath)

	// Check for local modifications when not forcing.
	if !force {
		statuses, err := e.Status(entry)
		if err != nil {
			return nil, fmt.Errorf("checking local status: %w", err)
		}
		for _, s := range statuses {
			if s.Status == StatusModified {
				return nil, fmt.Errorf("%w: %s has local modifications (use --force to overwrite)", wkerrors.ErrSyncConflict, s.FilePath)
			}
		}
	}

	// Resolve folder ID (cached when possible, ADR-005 Decision 9).
	folderID, err := e.folderIDForEntry(ctx, entry)
	if err != nil {
		return nil, e.wrapFolderErr(err, entry, entry.FolderID)
	}

	// Trigger export; invalidate cache and retry once on 404.
	origFolderID := folderID
	retried := false
	pkgID, err := e.packages.Export(ctx, folderID)
	if err != nil && entry.FolderID != 0 && invalidFolderCacheErr(err) {
		if fresh, rerr := e.resolveAndCache(ctx, entry.ServerPath); rerr == nil {
			folderID = fresh
			retried = true
			pkgID, err = e.packages.Export(ctx, folderID)
		} else {
			return nil, e.wrapFolderErr(rerr, entry, origFolderID)
		}
	}
	if err != nil {
		wrapID := origFolderID
		if retried {
			wrapID = 0 // retry used fresh ID; don't blame the stale cache
		}
		return nil, e.wrapFolderErr(err, entry, wrapID)
	}

	// Wait for export to complete.
	if err := e.waitForPackage(ctx, pkgID); err != nil {
		return nil, err
	}

	// Download the package zip.
	zipData, err := e.packages.Download(ctx, pkgID)
	if err != nil {
		return nil, fmt.Errorf("downloading package: %w", err)
	}

	// Extract and write files.
	return e.extractZip(zipData, localDir, entry.ServerPath)
}

// extractZip extracts a package zip into localDir, creating/updating meta files.
func (e *SyncEngine) extractZip(zipData []byte, localDir string, serverPath string) ([]PullResult, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}

	// Ensure local directory exists.
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory %s: %w", localDir, err)
	}

	ignore := e.ignoreMatcher()

	var results []PullResult

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opening %s in zip: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("reading %s in zip: %w", f.Name, err)
		}

		// Normalize zip path.
		relPath := filepath.ToSlash(f.Name)
		relPath = strings.TrimPrefix(relPath, "/")

		// Consult .wkignore using the project-root-relative path.
		absPath := filepath.Join(localDir, filepath.FromSlash(relPath))
		if projectRel, rerr := e.projectRel(absPath); rerr == nil {
			if ignore.ShouldSkip(projectRel, false) {
				results = append(results, PullResult{FilePath: relPath, Action: "skipped"})
				continue
			}
		}

		// Normalize JSON to prevent phantom diffs from server-side reformatting.
		if isJSON(relPath) {
			if normalized, err := normalizeJSON(data); err == nil {
				data = normalized
			}
		}

		newHash := ComputeHash(data)

		// Determine action.
		action := "created"
		if existing, err := os.ReadFile(absPath); err == nil {
			if ComputeHash(existing) == newHash {
				action = "unchanged"
			} else {
				action = "updated"
			}
		}

		if action != "unchanged" {
			// Ensure parent directory exists.
			if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
				return nil, fmt.Errorf("creating directory for %s: %w", absPath, err)
			}
			if err := os.WriteFile(absPath, data, 0644); err != nil {
				return nil, fmt.Errorf("writing %s: %w", absPath, err)
			}
		}

		// Write/update sidecar meta under .wk/.
		meta := &AssetMeta{
			ServerPath:   serverPath + "/" + relPath,
			ZipName:      f.Name,
			Folder:       filepath.Dir(relPath),
			Type:         inferAssetType(relPath),
			ContentHash:  newHash,
			LastPulledAt: time.Now().UTC(),
		}
		metaPath, err := MetaPath(e.projectRoot, absPath)
		if err != nil {
			return nil, fmt.Errorf("meta path for %s: %w", relPath, err)
		}
		if err := WriteMeta(metaPath, meta); err != nil {
			return nil, fmt.Errorf("writing meta for %s: %w", relPath, err)
		}

		results = append(results, PullResult{
			FilePath: relPath,
			Action:   action,
		})
	}

	return results, nil
}

// isJSON returns true if the file path has a .json extension.
func isJSON(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".json")
}

// normalizeJSON re-serializes JSON with sorted keys and consistent indentation.
func normalizeJSON(data []byte) ([]byte, error) {
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// inferAssetType guesses the asset type from a filename.
func inferAssetType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "recipe"):
		return "recipe"
	case strings.Contains(lower, "connection"):
		return "connection"
	case strings.HasSuffix(lower, ".api_endpoint.json"):
		return "api_endpoint"
	case strings.HasSuffix(lower, ".api_group.json"):
		return "api_collection"
	default:
		return "unknown"
	}
}
