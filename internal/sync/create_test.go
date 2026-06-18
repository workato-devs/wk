package sync

import (
	"context"
	"strings"
	"testing"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/config"
)

// createMockFolders mocks the Workato folders endpoint with a
// live-updating tree: List reads from tree, Create adds a new entry
// at the given parent and returns the assigned ID. Tests can inspect
// listCalls / createCalls to assert on API traffic.
type createMockFolders struct {
	tree        map[int][]api.Folder // parentID -> children (-1 = root)
	nextID      int
	listCalls   int
	createCalls int
	createErr   error
}

func newCreateMock() *createMockFolders {
	return &createMockFolders{
		tree:   map[int][]api.Folder{-1: nil},
		nextID: 100,
	}
}

func (m *createMockFolders) List(_ context.Context, parentID *int) ([]api.Folder, error) {
	m.listCalls++
	key := -1
	if parentID != nil {
		key = *parentID
	}
	return m.tree[key], nil
}

func (m *createMockFolders) Create(_ context.Context, name string, parentID *int) (*api.Folder, error) {
	m.createCalls++
	if m.createErr != nil {
		return nil, m.createErr
	}
	key := -1
	if parentID != nil {
		key = *parentID
	}
	folder := api.Folder{ID: m.nextID, Name: name, ParentID: parentID}
	// Mirror the Workato API: a create at root with no parent is a
	// project and gets a distinct project_id. Nested creates are plain
	// folders (project_id stays 0).
	if parentID == nil {
		folder.IsProject = true
		folder.ProjectID = m.nextID + 1000
	}
	m.nextID++
	m.tree[key] = append(m.tree[key], folder)
	return &folder, nil
}

func (m *createMockFolders) Delete(_ context.Context, _ int) error {
	return nil
}

func (m *createMockFolders) DeleteProject(_ context.Context, _ int) error {
	return nil
}

func newCreateEngine(cfg *config.Config, folders *createMockFolders) *SyncEngine {
	return &SyncEngine{
		config:  cfg,
		folders: folders,
	}
}

// TestCreateForEntry_BareNameDefault: default mode (CreateModeBareNames)
// creates a missing top-level folder on first push. One API create,
// one FolderCreated record, cache populated.
func TestCreateForEntry_BareNameDefault(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "MyProject"}},
	}
	folders := newCreateMock()
	engine := newCreateEngine(cfg, folders)

	id, err := engine.createForEntry(context.Background(), "MyProject")
	if err != nil {
		t.Fatalf("createForEntry: %v", err)
	}
	if id == 0 {
		t.Fatal("id = 0, want non-zero")
	}
	if folders.createCalls != 1 {
		t.Errorf("createCalls = %d, want 1", folders.createCalls)
	}
	if len(engine.foldersCreated) != 1 {
		t.Fatalf("foldersCreated len = %d, want 1", len(engine.foldersCreated))
	}
	if engine.foldersCreated[0].ServerPath != "MyProject" {
		t.Errorf("foldersCreated[0].ServerPath = %q, want MyProject", engine.foldersCreated[0].ServerPath)
	}
	if cfg.Sync[0].FolderID != id {
		t.Errorf("cfg.Sync[0].FolderID = %d, want %d (write-through cache)", cfg.Sync[0].FolderID, id)
	}
}

// TestCreateForEntry_NestedPathDefaultErrors: nested path + default
// mode errors loudly rather than silently creating a multi-level
// hierarchy (Decision 13 — nested miss is more likely a typo than
// intent).
func TestCreateForEntry_NestedPathDefaultErrors(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "Clients/Acme/Prod"}},
	}
	engine := newCreateEngine(cfg, newCreateMock())

	_, err := engine.createForEntry(context.Background(), "Clients/Acme/Prod")
	if err == nil {
		t.Fatal("err = nil, want error (nested path missing, default mode)")
	}
	if !strings.Contains(err.Error(), "--create-path") {
		t.Errorf("err = %v, want hint about --create-path", err)
	}
}

// TestCreateForEntry_NestedPathWithCreatePath: --create-path walks
// the tree and creates each missing segment. All three creates are
// recorded with their cumulative server_path.
func TestCreateForEntry_NestedPathWithCreatePath(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "Clients/Acme/Prod"}},
	}
	folders := newCreateMock()
	engine := newCreateEngine(cfg, folders)
	engine.SetCreateMode(CreateModeAnyPath)

	id, err := engine.createForEntry(context.Background(), "Clients/Acme/Prod")
	if err != nil {
		t.Fatalf("createForEntry: %v", err)
	}
	if id == 0 {
		t.Fatal("id = 0, want non-zero")
	}
	if folders.createCalls != 3 {
		t.Errorf("createCalls = %d, want 3", folders.createCalls)
	}
	if len(engine.foldersCreated) != 3 {
		t.Fatalf("foldersCreated len = %d, want 3", len(engine.foldersCreated))
	}
	wantPaths := []string{"Clients", "Clients/Acme", "Clients/Acme/Prod"}
	for i, want := range wantPaths {
		if engine.foldersCreated[i].ServerPath != want {
			t.Errorf("foldersCreated[%d].ServerPath = %q, want %q", i, engine.foldersCreated[i].ServerPath, want)
		}
	}
}

// TestCreateForEntry_NoCreateErrors: --no-create disables the fallback
// entirely — any missing folder surfaces as an error.
func TestCreateForEntry_NoCreateErrors(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "MyProject"}},
	}
	folders := newCreateMock()
	engine := newCreateEngine(cfg, folders)
	engine.SetCreateMode(CreateModeNever)

	_, err := engine.createForEntry(context.Background(), "MyProject")
	if err == nil {
		t.Fatal("err = nil, want error (--no-create set)")
	}
	if !strings.Contains(err.Error(), "--no-create") {
		t.Errorf("err = %v, want mention of --no-create", err)
	}
	if folders.createCalls != 0 {
		t.Errorf("createCalls = %d, want 0 (no-create must never call Create)", folders.createCalls)
	}
}

// TestCreateForEntry_AllProjectsPrefixStripped: "All projects/X" is a
// bare name after the prefix strip, so default mode still creates.
func TestCreateForEntry_AllProjectsPrefixStripped(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "All projects/MyProject"}},
	}
	folders := newCreateMock()
	engine := newCreateEngine(cfg, folders)

	id, err := engine.createForEntry(context.Background(), "All projects/MyProject")
	if err != nil {
		t.Fatalf("createForEntry: %v", err)
	}
	if id == 0 {
		t.Fatal("id = 0, want non-zero")
	}
	if folders.createCalls != 1 {
		t.Errorf("createCalls = %d, want 1", folders.createCalls)
	}
}

// TestCreateForEntry_UsesExistingParentWithCreatePath: when
// --create-path walks a path whose ancestors already exist, only the
// missing segments are created. Existing ones are reused.
func TestCreateForEntry_UsesExistingParentWithCreatePath(t *testing.T) {
	folders := newCreateMock()
	// Seed an existing "Clients" top-level so only "Acme" and "Prod"
	// need to be created.
	folders.tree[-1] = []api.Folder{{ID: 50, Name: "Clients"}}

	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "Clients/Acme/Prod"}},
	}
	engine := newCreateEngine(cfg, folders)
	engine.SetCreateMode(CreateModeAnyPath)

	_, err := engine.createForEntry(context.Background(), "Clients/Acme/Prod")
	if err != nil {
		t.Fatalf("createForEntry: %v", err)
	}
	if folders.createCalls != 2 {
		t.Errorf("createCalls = %d, want 2 (only Acme and Prod should be created)", folders.createCalls)
	}
	if len(engine.foldersCreated) != 2 {
		t.Fatalf("foldersCreated len = %d, want 2 (%+v)", len(engine.foldersCreated), engine.foldersCreated)
	}
}

// TestFolderIDForEntry_CreateFallbackIntegration verifies the end-to-end
// flow: uncached entry -> walk fails -> create -> ID returned + cached.
// This is the wiring that turns greenfield `wk init` + `wk push` into
// a working two-command flow (ADR-007 Decision 12).
func TestFolderIDForEntry_CreateFallbackIntegration(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "GreenfieldProject"}},
	}
	folders := newCreateMock()
	engine := newCreateEngine(cfg, folders)

	id, err := engine.folderIDForEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("folderIDForEntry: %v", err)
	}
	if id == 0 {
		t.Fatal("id = 0, want non-zero")
	}
	if folders.createCalls != 1 {
		t.Errorf("createCalls = %d, want 1", folders.createCalls)
	}
	if cfg.Sync[0].FolderID != id {
		t.Errorf("cfg.Sync[0].FolderID = %d, want %d", cfg.Sync[0].FolderID, id)
	}
}

// TestFolderIDForEntry_CacheHitSkipsCreate: cached entry is a fast
// path — no walk, no create, no API traffic.
func TestFolderIDForEntry_CacheHitSkipsCreate(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "Cached", FolderID: 42}},
	}
	folders := newCreateMock()
	engine := newCreateEngine(cfg, folders)

	id, err := engine.folderIDForEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("folderIDForEntry: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want cached 42", id)
	}
	if folders.listCalls != 0 || folders.createCalls != 0 {
		t.Errorf("listCalls=%d createCalls=%d, want both 0 (cache hit should be silent)", folders.listCalls, folders.createCalls)
	}
}

// TestFoldersCreated_NeverReturnsNil guards the JSON-serialization
// fix: an engine that never created anything should still return a
// non-nil (empty) slice so `folders_created` marshals as "[]" in the
// push JSON envelope, not "null".
func TestFoldersCreated_NeverReturnsNil(t *testing.T) {
	engine := newCreateEngine(&config.Config{}, newCreateMock())
	if got := engine.FoldersCreated(); got == nil {
		t.Fatal("FoldersCreated() = nil; want non-nil empty slice for JSON consumers")
	}
	if got := engine.FoldersCreated(); len(got) != 0 {
		t.Errorf("FoldersCreated() len = %d, want 0", len(got))
	}
}

// TestCreateForEntry_CapturesProjectID guards the project_id
// threading: when push creates a top-level project, both folder_id
// AND project_id must land in wk.toml so wk folders delete has the
// right value for DELETE /projects/{project_id}.
func TestCreateForEntry_CapturesProjectID(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "MyProject"}},
	}
	folders := newCreateMock()
	engine := newCreateEngine(cfg, folders)

	if _, err := engine.createForEntry(context.Background(), "MyProject"); err != nil {
		t.Fatalf("createForEntry: %v", err)
	}

	if cfg.Sync[0].FolderID == 0 {
		t.Errorf("cfg.Sync[0].FolderID = 0, want non-zero")
	}
	if cfg.Sync[0].ProjectID == 0 {
		t.Errorf("cfg.Sync[0].ProjectID = 0, want non-zero (project_id must be captured from Create response)")
	}
	if cfg.Sync[0].FolderID == cfg.Sync[0].ProjectID {
		t.Errorf("FolderID == ProjectID (%d); want distinct values", cfg.Sync[0].FolderID)
	}

	created := engine.FoldersCreated()
	if len(created) != 1 {
		t.Fatalf("FoldersCreated len = %d, want 1", len(created))
	}
	if created[0].ProjectID == 0 {
		t.Errorf("FoldersCreated[0].ProjectID = 0, want non-zero")
	}
}

// TestCreateForEntry_NestedFolderHasNoProjectID: nested creates are
// plain folders (not projects). project_id stays zero for them —
// only the top-level project segment should carry one.
func TestCreateForEntry_NestedFolderHasNoProjectID(t *testing.T) {
	cfg := &config.Config{
		Sync: []config.SyncEntry{{ServerPath: "Clients/Acme/Prod"}},
	}
	folders := newCreateMock()
	engine := newCreateEngine(cfg, folders)
	engine.SetCreateMode(CreateModeAnyPath)

	if _, err := engine.createForEntry(context.Background(), "Clients/Acme/Prod"); err != nil {
		t.Fatalf("createForEntry: %v", err)
	}

	created := engine.FoldersCreated()
	if len(created) != 3 {
		t.Fatalf("FoldersCreated len = %d, want 3", len(created))
	}
	// First segment is the project (created at root).
	if created[0].ProjectID == 0 {
		t.Errorf("top-level Clients should have project_id set")
	}
	// Nested segments are plain folders.
	for i, c := range created[1:] {
		if c.ProjectID != 0 {
			t.Errorf("nested segment %d (%s) got ProjectID=%d, want 0", i+1, c.ServerPath, c.ProjectID)
		}
	}

	// Leaf entry in cfg reflects the deepest folder, which is a plain
	// folder, so its ProjectID should be zero.
	if cfg.Sync[0].ProjectID != 0 {
		t.Errorf("cfg.Sync[0].ProjectID = %d, want 0 (leaf is nested, not a project)", cfg.Sync[0].ProjectID)
	}
}
