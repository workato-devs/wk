package sync

import (
	"context"
	"testing"

	"github.com/workato-devs/wk-cli-beta/internal/api"
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

func (m *mockFolderService) Get(_ context.Context, _ int) (*api.Folder, error) {
	return nil, nil
}

func (m *mockFolderService) Create(_ context.Context, _ string, _ *int) (*api.Folder, error) {
	return nil, nil
}

func (m *mockFolderService) Delete(_ context.Context, _ int) error {
	return nil
}

func newTestEngine(folders map[int][]api.Folder) *SyncEngine {
	return &SyncEngine{
		folders: &mockFolderService{folders: folders},
	}
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
