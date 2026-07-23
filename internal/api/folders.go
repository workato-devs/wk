package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type folderService struct {
	client *HTTPClient
}

func (s *folderService) List(ctx context.Context, parentID *int) ([]Folder, error) {
	params := url.Values{}
	if parentID != nil {
		params.Set("parent_id", strconv.Itoa(*parentID))
	}
	path := "/folders"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result []Folder
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *folderService) Create(ctx context.Context, name string, parentID *int) (*Folder, error) {
	body := map[string]any{"name": name}
	if parentID != nil {
		body["parent_id"] = *parentID
	}
	var folder Folder
	if err := s.client.do(ctx, "POST", "/folders", body, &folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

// Update renames a plain folder via PUT /folders/{id}. Projects use a
// separate endpoint — callers must route by Folder.IsProject and call
// UpdateProject instead (mirroring the Delete/DeleteProject split).
func (s *folderService) Update(ctx context.Context, id int, name string) (*Folder, error) {
	var folder Folder
	body := map[string]any{"name": name}
	if err := s.client.do(ctx, "PUT", fmt.Sprintf("/folders/%d", id), body, &folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

// UpdateProject renames a top-level project via PUT /projects/{id}. The
// id here is the project_id (distinct from the folder id), matching
// DeleteProject.
func (s *folderService) UpdateProject(ctx context.Context, projectID int, name string) (*Folder, error) {
	var folder Folder
	body := map[string]any{"name": name}
	if err := s.client.do(ctx, "PUT", fmt.Sprintf("/projects/%d", projectID), body, &folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

// ListProjects returns projects via their own endpoint (GET /projects),
// rather than inferring them from the folder list's is_project flag.
func (s *folderService) ListProjects(ctx context.Context) ([]Folder, error) {
	var result []Folder
	if err := s.client.do(ctx, "GET", "/projects", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *folderService) Delete(ctx context.Context, id int) error {
	return s.client.do(ctx, "DELETE", fmt.Sprintf("/folders/%d", id), nil, nil)
}

// DeleteProject removes a top-level project via DELETE /projects/{id}.
// The Workato folders DELETE endpoint does not handle projects;
// callers must route by Folder.IsProject.
func (s *folderService) DeleteProject(ctx context.Context, id int) error {
	return s.client.do(ctx, "DELETE", fmt.Sprintf("/projects/%d", id), nil, nil)
}
