package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type connectionService struct {
	client *HTTPClient
}

func (s *connectionService) List(ctx context.Context, opts *ConnectionListOptions) ([]Connection, error) {
	params := url.Values{}
	if opts != nil {
		if opts.FolderID != nil {
			params.Set("folder_id", strconv.Itoa(*opts.FolderID))
		}
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := "/connections"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result []Connection
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *connectionService) Get(ctx context.Context, id int) (*Connection, error) {
	// The Workato API has no single-connection GET endpoint.
	// Filter from the list instead.
	conns, err := s.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	for i := range conns {
		if conns[i].ID == id {
			return &conns[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: fmt.Sprintf("connection %d not found", id)}
}

