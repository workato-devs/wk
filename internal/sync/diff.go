package sync

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

// DiffType describes how a local asset differs from its remote counterpart.
type DiffType string

const (
	DiffAdded    DiffType = "added"    // exists remote, not local
	DiffModified DiffType = "modified" // both exist, different hash
	DiffDeleted  DiffType = "deleted"  // exists local, not remote
	DiffSame     DiffType = "same"     // identical
)

// DiffEntry describes the diff state of a single asset path.
type DiffEntry struct {
	Path       string   `json:"path"`
	Type       DiffType `json:"type"`
	LocalHash  string   `json:"local_hash,omitempty"`
	RemoteHash string   `json:"remote_hash,omitempty"`
}

// Diff compares local files against remote state for a sync entry.
// This requires API access to fetch the remote package.
func (e *SyncEngine) Diff(entry config.SyncEntry) ([]DiffEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Resolve folder ID from the server path.
	folderID, err := e.resolveFolderID(ctx, entry.ServerPath)
	if err != nil {
		return nil, fmt.Errorf("resolving folder %q: %w", entry.ServerPath, err)
	}

	// Trigger an export and wait for it to complete.
	remoteFiles, err := e.fetchRemoteFiles(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("fetching remote files: %w", err)
	}

	// Build map of local files with their hashes.
	localDir := filepath.Join(e.projectRoot, entry.LocalPath)
	localMetas, err := FindMetaFiles(localDir)
	if err != nil {
		return nil, err
	}

	// Also read current local file hashes for known assets.
	localHashes := make(map[string]string)
	for rel := range localMetas {
		absPath := filepath.Join(localDir, rel)
		data, err := readFileBytes(absPath)
		if err != nil {
			continue // file may have been deleted
		}
		localHashes[rel] = ComputeHash(data)
	}

	// Scan for files without meta (new local files).
	localOnly, err := findLocalFiles(localDir)
	if err != nil {
		return nil, err
	}
	for _, rel := range localOnly {
		if _, hasMeta := localMetas[rel]; hasMeta {
			continue
		}
		data, err := readFileBytes(filepath.Join(localDir, rel))
		if err != nil {
			continue
		}
		localHashes[rel] = ComputeHash(data)
	}

	// Compare.
	seen := make(map[string]bool)
	var results []DiffEntry

	for remotePath, remoteHash := range remoteFiles {
		seen[remotePath] = true
		localHash, exists := localHashes[remotePath]
		if !exists {
			results = append(results, DiffEntry{
				Path:       remotePath,
				Type:       DiffAdded,
				RemoteHash: remoteHash,
			})
			continue
		}
		if localHash == remoteHash {
			results = append(results, DiffEntry{
				Path:       remotePath,
				Type:       DiffSame,
				LocalHash:  localHash,
				RemoteHash: remoteHash,
			})
		} else {
			results = append(results, DiffEntry{
				Path:       remotePath,
				Type:       DiffModified,
				LocalHash:  localHash,
				RemoteHash: remoteHash,
			})
		}
	}

	// Files that exist locally but not remotely.
	for localPath, localHash := range localHashes {
		if seen[localPath] {
			continue
		}
		results = append(results, DiffEntry{
			Path:      localPath,
			Type:      DiffDeleted,
			LocalHash: localHash,
		})
	}

	return results, nil
}

// fetchRemoteFiles exports the remote folder and returns a map of
// relative file path -> content hash for every file in the zip.
func (e *SyncEngine) fetchRemoteFiles(ctx context.Context, folderID int) (map[string]string, error) {
	pkgID, err := e.packages.Export(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("starting export: %w", err)
	}

	// Poll until export completes.
	if err := e.waitForPackage(ctx, pkgID); err != nil {
		return nil, err
	}

	data, err := e.packages.Download(ctx, pkgID)
	if err != nil {
		return nil, fmt.Errorf("downloading package: %w", err)
	}

	return hashZipContents(data)
}

// hashZipContents reads a zip archive and returns a map of filename -> SHA256 hash.
func hashZipContents(zipData []byte) (map[string]string, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}

	result := make(map[string]string)
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
		// Normalize the zip path to a clean relative path.
		name := filepath.ToSlash(f.Name)
		name = strings.TrimPrefix(name, "/")
		result[name] = ComputeHash(data)
	}
	return result, nil
}
