package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/workato-devs/wk-cli-beta/internal/config"
)

// RefreshState is the per-entry outcome of `wk sync refresh` (ADR-007
// Decision 11). Values are stable across human and JSON output so
// downstream scripts can branch on the string literally.
type RefreshState string

const (
	// RefreshStateFound: entry had no cached folder_id and the walk
	// succeeded. First-time resolution — no drift involved. The new ID
	// has been written into the in-memory config slot; callers save
	// once per sweep.
	RefreshStateFound RefreshState = "found"

	// RefreshStateCurrent: entry had a cached folder_id and the walk
	// returned the same ID. No cache change needed.
	RefreshStateCurrent RefreshState = "current"

	// RefreshStateRepaired: entry had a cached folder_id but the walk
	// returned a different ID (renamed folder, recreated with a new
	// ID, workspace swap, etc.). Drift was detected; the cache has
	// been rewritten with the fresh ID. CI monitors can alert on this
	// state to see that the workspace changed underfoot.
	RefreshStateRepaired RefreshState = "repaired"

	// RefreshStateNotFound: walk failed — server_path does not describe
	// a real folder. Reported; --prune removes. Entries that had a
	// cached folder_id still show it in the output so the developer can
	// see "used to work" vs "never worked"; the distinction is in the
	// data, not in a separate state label.
	RefreshStateNotFound RefreshState = "not-found"
)

// RefreshResult is the per-entry outcome emitted by ClassifyEntry.
// ServerPath / LocalPath come from the incoming entry; FolderID is
// either the resolved ID (found / current / repaired) or the cached ID
// from before classification (not-found, so the developer can still
// see what the local wk.toml had). Message carries a human detail
// string for repaired / not-found; empty for found / current.
type RefreshResult struct {
	ServerPath string       `json:"server_path"`
	LocalPath  string       `json:"local_path"`
	FolderID   int          `json:"folder_id,omitempty"`
	State      RefreshState `json:"state"`
	Message    string       `json:"message,omitempty"`
}

// ClassifyEntry reconciles one [[sync]] entry against current server
// state (ADR-007 Decision 11). The Workato API does not expose a
// single-folder-by-ID endpoint, so cache validation happens by walking
// the hierarchy via List and comparing the resolved leaf ID against
// the cached value. On a fresh resolution (found) or cache repair
// (repaired), writes the new folder_id into the matching config slot
// in memory; callers persist via config.Save once the full sweep has
// run.
//
// Returns a non-nil error only for genuine API failures (auth, 5xx,
// network). "Name not present under expected parent" is distinguished
// from API failure via the errPathNotResolved sentinel that
// resolveFolderID wraps into its error, and classifies the entry as
// not-found rather than halting the sweep.
func (e *SyncEngine) ClassifyEntry(ctx context.Context, entry config.SyncEntry) (RefreshResult, error) {
	result := RefreshResult{
		ServerPath: entry.ServerPath,
		LocalPath:  entry.LocalPath,
		FolderID:   entry.FolderID,
	}

	id, err := e.resolveFolderID(ctx, entry.ServerPath)
	if err != nil {
		if !errors.Is(err, errPathNotResolved) {
			return RefreshResult{}, fmt.Errorf("resolving %q: %w", entry.ServerPath, err)
		}
		result.State = RefreshStateNotFound
		if entry.FolderID != 0 {
			result.Message = fmt.Sprintf("cached folder_id=%d no longer resolves", entry.FolderID)
		} else {
			result.Message = fmt.Sprintf("server path %q does not resolve", entry.ServerPath)
		}
		return result, nil
	}

	if entry.FolderID == 0 {
		result.State = RefreshStateFound
		result.FolderID = id
		e.writeCachedFolderID(entry.ServerPath, id)
		return result, nil
	}
	if entry.FolderID == id {
		result.State = RefreshStateCurrent
		return result, nil
	}

	result.State = RefreshStateRepaired
	result.FolderID = id
	result.Message = fmt.Sprintf("cached folder_id changed from %d to %d", entry.FolderID, id)
	e.writeCachedFolderID(entry.ServerPath, id)
	return result, nil
}

// writeCachedFolderID updates the in-memory config slot for the
// matching [[sync]] entry. Callers batch config.Save once per sweep.
func (e *SyncEngine) writeCachedFolderID(serverPath string, id int) {
	if e.config == nil {
		return
	}
	for i := range e.config.Sync {
		if e.config.Sync[i].ServerPath == serverPath {
			e.config.Sync[i].FolderID = id
			return
		}
	}
}
