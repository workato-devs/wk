package api

import "time"

// Recipe represents a Workato recipe.
type Recipe struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	FolderID    int       `json:"folder_id"`
	Running     bool      `json:"running"`
	Active      bool      `json:"active"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Code        any       `json:"code,omitempty"`
	Config      any       `json:"config,omitempty"`
}

// Connection represents a Workato connection.
type Connection struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	Application         string    `json:"application"`
	FolderID            int       `json:"folder_id"`
	AuthorizationStatus *string   `json:"authorization_status"`
	AuthorizationError  *string   `json:"authorization_error"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Folder represents a Workato folder.
type Folder struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID *int   `json:"parent_id,omitempty"`
}

// Package represents an RLCM export/import package.
type Package struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExportManifest represents a Workato RLCM export manifest.
// Creating a manifest is required before triggering a package export.
type ExportManifest struct {
	ID       int    `json:"id"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status,omitempty"`
	FolderID int    `json:"folder_id,omitempty"`
}

// PackageContent describes a single asset within an RLCM package.
type PackageContent struct {
	AbsolutePath string `json:"absolute_path"`
	ZipName      string `json:"zip_name"`
	Folder       string `json:"folder"`
	Type         string `json:"type"` // "recipe", "connection", etc.
}

// Job represents a recipe job execution.
type Job struct {
	ID          int        `json:"id"`
	RecipeID    int        `json:"recipe_id"`
	Status      string     `json:"status"` // "succeeded", "failed", "pending"
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ListResult is a generic wrapper for paginated API responses.
// Used by recipes (which return {"items":[...]}) but not connections/folders (which return bare arrays).
type ListResult[T any] struct {
	Items []T `json:"items"`
}
