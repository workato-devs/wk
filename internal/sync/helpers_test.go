package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/config"
)

type mockFolderService struct {
	folders map[int][]api.Folder // parentID -> children (use -1 for nil/root)
}

func (m *mockFolderService) List(_ context.Context, parentID *int) ([]api.Folder, error) {
	key := -1
	if parentID != nil {
		key = *parentID
	}
	return m.folders[key], nil
}

func (m *mockFolderService) Create(_ context.Context, _ string, _ *int) (*api.Folder, error) {
	return nil, nil
}

func (m *mockFolderService) Delete(_ context.Context, _ int) error {
	return nil
}

func (m *mockFolderService) DeleteProject(_ context.Context, _ int) error {
	return nil
}

func newTestEngine(folders map[int][]api.Folder) *SyncEngine {
	return &SyncEngine{
		folders: &mockFolderService{folders: folders},
	}
}

// TestFolderIDForEntry_UsesCache verifies the fast-path: when FolderID is
// set on the entry, no folder-service call happens.
func TestFolderIDForEntry_UsesCache(t *testing.T) {
	// Empty folder service — a path resolution would error.
	engine := newTestEngine(map[int][]api.Folder{})
	entry := config.SyncEntry{ServerPath: "A", FolderID: 42}

	id, err := engine.folderIDForEntry(context.Background(), entry)
	if err != nil {
		t.Fatalf("folderIDForEntry: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want cached 42", id)
	}
}

// TestFolderIDForEntry_WriteThrough verifies that a cache miss triggers
// resolution and persists the resulting ID back to wk.toml.
func TestFolderIDForEntry_WriteThrough(t *testing.T) {
	root := t.TempDir()
	// Pre-create .wk/ and write a config with one entry and no FolderID.
	if err := writeConfigForTest(root, &config.Config{
		Name: "t",
		Sync: []config.SyncEntry{{ServerPath: "MyProject"}},
	}); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	cfg, err := config.Load(config.ProjectConfigPath(root))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	engine := &SyncEngine{
		projectRoot: root,
		config:      cfg,
		folders: &mockFolderService{folders: map[int][]api.Folder{
			-1: {{ID: 100, Name: "MyProject"}},
		}},
	}

	id, err := engine.folderIDForEntry(context.Background(), cfg.Sync[0])
	if err != nil {
		t.Fatalf("folderIDForEntry: %v", err)
	}
	if id != 100 {
		t.Errorf("id = %d, want 100", id)
	}

	// Reload from disk; the FolderID should have been persisted.
	reloaded, err := config.Load(config.ProjectConfigPath(root))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Sync[0].FolderID != 100 {
		t.Errorf("persisted FolderID = %d, want 100", reloaded.Sync[0].FolderID)
	}
}

func writeConfigForTest(root string, cfg *config.Config) error {
	if err := os.MkdirAll(filepath.Join(root, config.ProjectDir), 0755); err != nil {
		return err
	}
	return config.Save(config.ProjectConfigPath(root), cfg)
}

func TestResolveFolderID(t *testing.T) {
	engine := newTestEngine(map[int][]api.Folder{
		-1: {
			{ID: 100, Name: "MyProject"},
			{ID: 200, Name: "A"},
		},
		200: {
			{ID: 300, Name: "B"},
		},
	})

	tests := []struct {
		name       string
		serverPath string
		wantID     int
		wantErr    bool
	}{
		{
			name:       "All projects prefix strips to root child",
			serverPath: "All projects/MyProject",
			wantID:     100,
		},
		{
			name:       "direct root child without prefix",
			serverPath: "MyProject",
			wantID:     100,
		},
		{
			name:       "All projects alone returns root",
			serverPath: "All projects",
			wantID:     0,
		},
		{
			name:       "nested path with All projects prefix",
			serverPath: "All projects/A/B",
			wantID:     300,
		},
		{
			name:       "nonexistent folder returns error",
			serverPath: "NonExistent",
			wantErr:    true,
		},
		{
			name:       "empty path returns error",
			serverPath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := engine.resolveFolderID(context.Background(), tt.serverPath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%d", id)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Errorf("got id=%d, want %d", id, tt.wantID)
			}
		})
	}
}
