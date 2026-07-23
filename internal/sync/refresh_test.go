package sync

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/config"
)

// refreshMockFolders lets each test case stub List behavior per-parent
// and, when needed, inject a transient error to simulate API failure.
// listCalls counts the total List invocations so tests can assert the
// folder-list cache actually memoizes across entries.
type refreshMockFolders struct {
	list      map[int][]api.Folder
	listErr   error
	listCalls int
}

func (m *refreshMockFolders) List(_ context.Context, parentID *int) ([]api.Folder, error) {
	m.listCalls++
	if m.listErr != nil {
		return nil, m.listErr
	}
	key := -1
	if parentID != nil {
		key = *parentID
	}
	return m.list[key], nil
}

func (m *refreshMockFolders) Create(_ context.Context, _ string, _ *int) (*api.Folder, error) {
	return nil, nil
}

func (m *refreshMockFolders) Delete(_ context.Context, _ int) error {
	return nil
}

func (m *refreshMockFolders) DeleteProject(_ context.Context, _ int) error {
	return nil
}

func (m *refreshMockFolders) ListProjects(_ context.Context) ([]api.Folder, error) {
	return nil, nil
}

func (m *refreshMockFolders) Update(_ context.Context, _ int, _ string) (*api.Folder, error) {
	return nil, nil
}

func (m *refreshMockFolders) UpdateProject(_ context.Context, _ int, _ string) (*api.Folder, error) {
	return nil, nil
}

func newRefreshEngine(cfg *config.Config, folders *refreshMockFolders) *SyncEngine {
	return &SyncEngine{
		config:  cfg,
		folders: folders,
	}
}

// TestClassifyEntry_Found_UncachedWritesCache: entry starts with no
// folder_id, the walk succeeds. First-time resolution — state is
// "found", cache is populated.
func TestClassifyEntry_Found_UncachedWritesCache(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "MyProject"}},
	}
	engine := newRefreshEngine(cfg, &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {{ID: 100, Name: "MyProject"}},
		},
	})

	result, err := engine.ClassifyEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("ClassifyEntry: %v", err)
	}
	if result.State != RefreshStateFound {
		t.Errorf("state = %q, want %q", result.State, RefreshStateFound)
	}
	if result.FolderID != 100 {
		t.Errorf("result.FolderID = %d, want 100", result.FolderID)
	}
	if cfg.Sync[0].FolderID != 100 {
		t.Errorf("cfg.Sync[0].FolderID = %d, want 100 (in-memory write-through)", cfg.Sync[0].FolderID)
	}
}

// TestClassifyEntry_Repaired_CachedMismatchRewritesCache: entry has a
// cached folder_id but the walk returns a different ID (e.g. folder
// was renamed or recreated). State is "repaired" — drift detected
// and auto-healed. Cache is rewritten.
func TestClassifyEntry_Repaired_CachedMismatchRewritesCache(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "MyProject", FolderID: 77}},
	}
	engine := newRefreshEngine(cfg, &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {{ID: 100, Name: "MyProject"}},
		},
	})

	result, err := engine.ClassifyEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("ClassifyEntry: %v", err)
	}
	if result.State != RefreshStateRepaired {
		t.Errorf("state = %q, want %q", result.State, RefreshStateRepaired)
	}
	if result.FolderID != 100 {
		t.Errorf("result.FolderID = %d, want 100", result.FolderID)
	}
	if !strings.Contains(result.Message, "77") || !strings.Contains(result.Message, "100") {
		t.Errorf("message %q should mention both old (77) and new (100) IDs", result.Message)
	}
	if cfg.Sync[0].FolderID != 100 {
		t.Errorf("cfg.Sync[0].FolderID = %d, want 100 (cache overwrote stale 77)", cfg.Sync[0].FolderID)
	}
}

// TestClassifyEntry_Current_WalkMatchesCachedID: entry has a cached
// folder_id AND the walk returns the same ID. No cache change needed.
func TestClassifyEntry_Current_WalkMatchesCachedID(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "MyProject", FolderID: 100}},
	}
	engine := newRefreshEngine(cfg, &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {{ID: 100, Name: "MyProject"}},
		},
	})

	result, err := engine.ClassifyEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("ClassifyEntry: %v", err)
	}
	if result.State != RefreshStateCurrent {
		t.Errorf("state = %q, want %q", result.State, RefreshStateCurrent)
	}
	if result.FolderID != 100 {
		t.Errorf("result.FolderID = %d, want 100", result.FolderID)
	}
}

// TestClassifyEntry_NotFound_CachedButWalkFails: entry has a cached
// folder_id but the hierarchy no longer contains the path. Reported
// as not-found with the cached id preserved in the output so the
// developer can see "used to work, now gone". Cache is NOT mutated.
func TestClassifyEntry_NotFound_CachedButWalkFails(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "Gone", FolderID: 999}},
	}
	engine := newRefreshEngine(cfg, &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {{ID: 100, Name: "RealProject"}},
		},
	})

	result, err := engine.ClassifyEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("ClassifyEntry: %v (not-found is a result, not an error)", err)
	}
	if result.State != RefreshStateNotFound {
		t.Errorf("state = %q, want %q", result.State, RefreshStateNotFound)
	}
	if result.FolderID != 999 {
		t.Errorf("result.FolderID = %d, want 999 (cached id preserved in output)", result.FolderID)
	}
	if !strings.Contains(result.Message, "999") {
		t.Errorf("message %q should mention the cached id", result.Message)
	}
	if cfg.Sync[0].FolderID != 999 {
		t.Errorf("cfg.Sync[0].FolderID = %d, want untouched 999", cfg.Sync[0].FolderID)
	}
}

// TestClassifyEntry_NotFound_UncachedWalkFails: uncached entry whose
// server_path does not describe a real folder. Reported with
// folder_id=0 (nothing to report there) so the output differentiates
// "never resolved" from "used to resolve".
func TestClassifyEntry_NotFound_UncachedWalkFails(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "Ghost"}},
	}
	engine := newRefreshEngine(cfg, &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {{ID: 100, Name: "RealProject"}},
		},
	})

	result, err := engine.ClassifyEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("ClassifyEntry: %v (not-found is a result, not an error)", err)
	}
	if result.State != RefreshStateNotFound {
		t.Errorf("state = %q, want %q", result.State, RefreshStateNotFound)
	}
	if result.FolderID != 0 {
		t.Errorf("result.FolderID = %d, want 0 (no cache to preserve)", result.FolderID)
	}
	if cfg.Sync[0].FolderID != 0 {
		t.Errorf("cfg.Sync[0].FolderID = %d, want untouched 0", cfg.Sync[0].FolderID)
	}
}

// TestClassifyEntry_PropagatesAPIErrors confirms that API-level failures
// (auth, 5xx, network) halt the sweep instead of being misclassified
// as not-found. Only "name not present under expected parent" maps to
// not-found.
func TestClassifyEntry_PropagatesAPIErrors(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "Anything"}},
	}
	transient := errors.New("connection reset by peer")
	engine := newRefreshEngine(cfg, &refreshMockFolders{
		listErr: transient,
	})

	_, err := engine.ClassifyEntry(context.Background(), cfg.Sync[0])
	if err == nil {
		t.Fatal("err = nil, want non-nil for API failure")
	}
	if !errors.Is(err, transient) {
		t.Errorf("err = %v, want to wrap transient %v", err, transient)
	}
}

// TestClassifyEntry_ListCacheMemoizesAcrossEntries guards the perf fix
// for `wk sync refresh`: N entries in the same workspace share the
// top-level folder list, so enabling the list cache should collapse N
// List calls into 1.
func TestClassifyEntry_ListCacheMemoizesAcrossEntries(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{
			{ServerPath: "Alpha"},
			{ServerPath: "Beta"},
			{ServerPath: "Gamma"},
		},
	}
	folders := &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {
				{ID: 1, Name: "Alpha"},
				{ID: 2, Name: "Beta"},
				{ID: 3, Name: "Gamma"},
			},
		},
	}
	engine := newRefreshEngine(cfg, folders)
	engine.EnableFolderListCache()

	for i := range cfg.Sync {
		if _, err := engine.ClassifyEntry(context.Background(), cfg.Sync[i]); err != nil {
			t.Fatalf("ClassifyEntry[%d]: %v", i, err)
		}
	}
	if folders.listCalls != 1 {
		t.Errorf("listCalls = %d, want 1 (cache should collapse 3 entries into one List)", folders.listCalls)
	}
}

// TestClassifyEntry_ListCacheDisabledCallsEachTime pins the default —
// normal operations (pull/push/status) should NOT share cached lists
// because they can see mid-run server changes.
func TestClassifyEntry_ListCacheDisabledCallsEachTime(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{
			{ServerPath: "Alpha"},
			{ServerPath: "Beta"},
		},
	}
	folders := &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {
				{ID: 1, Name: "Alpha"},
				{ID: 2, Name: "Beta"},
			},
		},
	}
	engine := newRefreshEngine(cfg, folders)

	for i := range cfg.Sync {
		if _, err := engine.ClassifyEntry(context.Background(), cfg.Sync[i]); err != nil {
			t.Fatalf("ClassifyEntry[%d]: %v", i, err)
		}
	}
	if folders.listCalls != 2 {
		t.Errorf("listCalls = %d, want 2 (cache disabled → one List per entry)", folders.listCalls)
	}
}

// TestClassifyEntry_CapturesProjectID: when a `found` classification
// populates the cache, project_id must land alongside folder_id in
// wk.toml. wk folders delete depends on project_id being present.
func TestClassifyEntry_CapturesProjectID(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "MyProject"}},
	}
	engine := newRefreshEngine(cfg, &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {{ID: 100, Name: "MyProject", IsProject: true, ProjectID: 9001}},
		},
	})

	result, err := engine.ClassifyEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("ClassifyEntry: %v", err)
	}
	if result.State != RefreshStateFound {
		t.Errorf("state = %q, want %q", result.State, RefreshStateFound)
	}
	if result.FolderID != 100 {
		t.Errorf("result.FolderID = %d, want 100", result.FolderID)
	}
	if result.ProjectID != 9001 {
		t.Errorf("result.ProjectID = %d, want 9001", result.ProjectID)
	}
	if cfg.Sync[0].FolderID != 100 || cfg.Sync[0].ProjectID != 9001 {
		t.Errorf("cfg.Sync[0] = {%d, %d}, want {100, 9001}", cfg.Sync[0].FolderID, cfg.Sync[0].ProjectID)
	}
}

// TestClassifyEntry_CurrentRefreshesProjectIDOnChange: even when the
// folder_id is unchanged, a stale project_id must be refreshed. This
// case is rare but possible — the server can flip is_project on a
// folder without changing its id.
func TestClassifyEntry_CurrentRefreshesProjectIDOnChange(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "MyProject", FolderID: 100, ProjectID: 0}},
	}
	engine := newRefreshEngine(cfg, &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {{ID: 100, Name: "MyProject", IsProject: true, ProjectID: 9001}},
		},
	})

	result, err := engine.ClassifyEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("ClassifyEntry: %v", err)
	}
	if result.State != RefreshStateCurrent {
		t.Errorf("state = %q, want %q", result.State, RefreshStateCurrent)
	}
	if cfg.Sync[0].ProjectID != 9001 {
		t.Errorf("cfg.Sync[0].ProjectID = %d, want 9001 (refreshed even on current)", cfg.Sync[0].ProjectID)
	}
}

// TestClassifyEntry_AllProjectsStripped covers the special-case root
// path — uncached "All projects/..." still classifies as found and
// writes the leaf ID.
func TestClassifyEntry_AllProjectsStripped(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "All projects/MyProject"}},
	}
	engine := newRefreshEngine(cfg, &refreshMockFolders{
		list: map[int][]api.Folder{
			-1: {{ID: 100, Name: "MyProject"}},
		},
	})

	result, err := engine.ClassifyEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("ClassifyEntry: %v", err)
	}
	if result.State != RefreshStateFound {
		t.Errorf("state = %q, want %q", result.State, RefreshStateFound)
	}
	if result.FolderID != 100 {
		t.Errorf("FolderID = %d, want 100", result.FolderID)
	}
}
