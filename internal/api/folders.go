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

// Move relocates a plain folder to a new parent via PUT /folders/{id} with
// an explicit parent_id. There is no dedicated update endpoint on this
// baseline yet, but Move is still kept as its own explicit method rather
// than folded into a general update, matching Recipes().Move, which is a
// separate command from Recipes().Update for the same reason: a structural
// change should never happen as a side effect of something else.
// Only plain folders are supported. Projects are top-level by definition
// and moving one is a separate, unverified question.
func (s *folderService) Move(ctx context.Context, id int, parentID int) (*Folder, error) {
	var folder Folder
	body := map[string]any{"parent_id": parentID}
	if err := s.client.do(ctx, "PUT", fmt.Sprintf("/folders/%d", id), body, &folder); err != nil {
		return nil, err
	}
	return &folder, nil
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
