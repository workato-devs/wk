package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type apiCollectionService struct {
	client *HTTPClient
}

func (s *apiCollectionService) List(ctx context.Context, opts *PaginationOptions) ([]APICollection, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := "/api_collections"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result []APICollection
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *apiCollectionService) Create(ctx context.Context, name string, projectID *int) (*APICollection, error) {
	body := map[string]any{
		"name": name,
	}
	if projectID != nil {
		body["project_id"] = *projectID
	}
	var collection APICollection
	if err := s.client.do(ctx, "POST", "/api_collections", body, &collection); err != nil {
		return nil, err
	}
	return &collection, nil
}

func (s *apiCollectionService) Delete(ctx context.Context, id int) error {
	return s.client.do(ctx, "DELETE", fmt.Sprintf("/api_collections/%d", id), nil, nil)
}
