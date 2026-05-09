package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type apiEndpointService struct {
	client *HTTPClient
}

func (s *apiEndpointService) List(ctx context.Context, collectionID *int, opts *PaginationOptions) ([]APIEndpoint, error) {
	params := url.Values{}
	if collectionID != nil {
		params.Set("api_collection_id", strconv.Itoa(*collectionID))
	}
	if opts != nil {
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := "/api_endpoints"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result []APIEndpoint
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *apiEndpointService) Create(ctx context.Context, collectionID int, data []byte) (*APIEndpoint, error) {
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("invalid endpoint JSON: %w", err)
	}
	var ep APIEndpoint
	if err := s.client.do(ctx, "POST", fmt.Sprintf("/api_collections/%d/api_endpoints", collectionID), body, &ep); err != nil {
		return nil, err
	}
	if ep.RecipeID == 0 && ep.FlowID != 0 {
		ep.RecipeID = ep.FlowID
	}
	return &ep, nil
}

func (s *apiEndpointService) Enable(ctx context.Context, id int) error {
	return s.client.do(ctx, "PUT", fmt.Sprintf("/api_endpoints/%d/enable", id), nil, nil)
}

func (s *apiEndpointService) Disable(ctx context.Context, id int) error {
	return s.client.do(ctx, "PUT", fmt.Sprintf("/api_endpoints/%d/disable", id), nil, nil)
}
