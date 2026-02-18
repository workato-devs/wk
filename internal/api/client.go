package api

import "context"

// RecipeService defines operations on recipes.
type RecipeService interface {
	List(ctx context.Context, opts *RecipeListOptions) ([]Recipe, error)
	Get(ctx context.Context, id int) (*Recipe, error)
	Start(ctx context.Context, id int) error
	Stop(ctx context.Context, id int) error
	Export(ctx context.Context, id int) ([]byte, error)
	Import(ctx context.Context, folderID int, data []byte) (*Recipe, error)
}

// RecipeListOptions configures recipe list filtering.
type RecipeListOptions struct {
	FolderID *int
	Status   string // "running", "stopped", "all"
	Page     int
	PerPage  int
}

// ConnectionService defines operations on connections.
type ConnectionService interface {
	List(ctx context.Context, opts *ConnectionListOptions) ([]Connection, error)
	Get(ctx context.Context, id int) (*Connection, error)
}

// ConnectionListOptions configures connection list filtering.
type ConnectionListOptions struct {
	FolderID *int
	Page     int
	PerPage  int
}

// FolderService defines operations on folders.
type FolderService interface {
	List(ctx context.Context, parentID *int) ([]Folder, error)
	Get(ctx context.Context, id int) (*Folder, error)
	Create(ctx context.Context, name string, parentID *int) (*Folder, error)
}

// PackageService defines operations on RLCM packages (export/import).
type PackageService interface {
	Export(ctx context.Context, folderID int) (int, error)              // returns package ID
	ExportStatus(ctx context.Context, packageID int) (*Package, error)
	Download(ctx context.Context, packageID int) ([]byte, error)
	Import(ctx context.Context, folderID int, data []byte, restartRecipes bool) (int, error) // returns import ID
	ImportStatus(ctx context.Context, importID int) (*Package, error)
}

// Client is the top-level API client providing access to all services.
type Client interface {
	Recipes() RecipeService
	Connections() ConnectionService
	Folders() FolderService
	Packages() PackageService
}
