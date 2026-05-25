package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type apiClientService struct {
	client *HTTPClient
}

func (s *apiClientService) List(ctx context.Context, opts *PaginationOptions) ([]APIClient, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := "/v2/api_clients"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var wrapper struct {
		Data []APIClient `json:"data"`
	}
	if err := s.client.do(ctx, "GET", path, nil, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data, nil
}

func (s *apiClientService) Get(ctx context.Context, id int) (*APIClient, error) {
	var wrapper struct {
		Data APIClient `json:"data"`
	}
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/v2/api_clients/%d", id), nil, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Data, nil
}

func (s *apiClientService) Create(ctx context.Context, name string, collectionIDs []string, authType string) (*APIClient, error) {
	body := map[string]any{
		"name": name,
	}
	if len(collectionIDs) > 0 {
		body["api_collection_ids"] = collectionIDs
	}
	if authType != "" {
		body["auth_type"] = authType
	}
	var wrapper struct {
		Data APIClient `json:"data"`
	}
	if err := s.client.do(ctx, "POST", "/v2/api_clients", body, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Data, nil
}

func (s *apiClientService) Delete(ctx context.Context, id int) error {
	return s.client.do(ctx, "DELETE", fmt.Sprintf("/v2/api_clients/%d", id), nil, nil)
}

func (s *apiClientService) CreateKey(ctx context.Context, clientID int, name string) (*APIKey, error) {
	body := map[string]any{
		"name":   name,
		"active": true,
	}
	var wrapper struct {
		Data APIKey `json:"data"`
	}
	path := fmt.Sprintf("/v2/api_clients/%d/api_keys", clientID)
	if err := s.client.do(ctx, "POST", path, body, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Data, nil
}

func (s *apiClientService) RefreshKey(ctx context.Context, clientID, keyID int) (*APIKey, error) {
	var wrapper struct {
		Data APIKey `json:"data"`
	}
	path := fmt.Sprintf("/v2/api_clients/%d/api_keys/%d/refresh_secret", clientID, keyID)
	if err := s.client.do(ctx, "PUT", path, nil, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Data, nil
}
