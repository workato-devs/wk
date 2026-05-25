package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type mcpServerService struct {
	client *HTTPClient
}

func (s *mcpServerService) List(ctx context.Context, opts *MCPServerListOptions) ([]MCPManagedServer, error) {
	params := url.Values{}
	if opts != nil {
		if opts.ProjectID != nil {
			params.Set("project_id", strconv.Itoa(*opts.ProjectID))
		}
		if opts.FolderID != nil {
			params.Set("folder_id", strconv.Itoa(*opts.FolderID))
		}
		if opts.AuthenticationMethod != "" {
			params.Set("authentication_method", opts.AuthenticationMethod)
		}
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := "/mcp/mcp_servers"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var wrapper struct {
		Data []MCPManagedServer `json:"data"`
	}
	if err := s.client.do(ctx, "GET", path, nil, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data, nil
}

func (s *mcpServerService) Get(ctx context.Context, handle string) (*MCPManagedServer, error) {
	var wrapper struct {
		Data MCPManagedServer `json:"data"`
	}
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/mcp/mcp_servers/%s", handle), nil, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Data, nil
}

func (s *mcpServerService) Create(ctx context.Context, name string, folderID int, description string, assetID *int) (*MCPManagedServer, error) {
	body := map[string]any{
		"name":      name,
		"folder_id": folderID,
	}
	if description != "" {
		body["description"] = description
	}
	if assetID != nil {
		body["asset_id"] = *assetID
	}
	var wrapper struct {
		Data MCPManagedServer `json:"data"`
	}
	if err := s.client.do(ctx, "POST", "/mcp/mcp_servers", body, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Data, nil
}

func (s *mcpServerService) Update(ctx context.Context, handle string, opts map[string]any) (*MCPManagedServer, error) {
	var wrapper struct {
		Data MCPManagedServer `json:"data"`
	}
	if err := s.client.do(ctx, "PUT", fmt.Sprintf("/mcp/mcp_servers/%s", handle), opts, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Data, nil
}

func (s *mcpServerService) Delete(ctx context.Context, handle string) error {
	return s.client.do(ctx, "DELETE", fmt.Sprintf("/mcp/mcp_servers/%s", handle), nil, nil)
}

func (s *mcpServerService) TokenRenew(ctx context.Context, handle string) (*MCPManagedServer, error) {
	var wrapper struct {
		Data MCPManagedServer `json:"data"`
	}
	if err := s.client.do(ctx, "POST", fmt.Sprintf("/mcp/mcp_servers/%s/token_renew", handle), nil, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Data, nil
}

func (s *mcpServerService) ListTools(ctx context.Context, handle string, opts *PaginationOptions) ([]MCPServerTool, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := fmt.Sprintf("/mcp/mcp_servers/%s/tools", handle)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	// Tools endpoint returns {"items":[...]} not {"data":[...]}
	var wrapper struct {
		Items []MCPServerTool `json:"items"`
	}
	if err := s.client.do(ctx, "GET", path, nil, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Items, nil
}
