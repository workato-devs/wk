package api

import (
	"context"
	"net/url"
)

type connectorService struct {
	client *HTTPClient
}

func (s *connectorService) List(ctx context.Context, search string) ([]Connector, error) {
	params := url.Values{}
	if search != "" {
		params.Set("applications", search)
	}
	path := "/integrations"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result []Connector
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
