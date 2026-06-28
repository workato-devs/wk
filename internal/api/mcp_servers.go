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

// AssignTools attaches tools to a server via POST .../assign_tools. Each tool
// is a {"trigger_application": ..., "id": ...} descriptor; for a recipe-backed
// tool the trigger_application is the recipe's own trigger connector.
func (s *mcpServerService) AssignTools(ctx context.Context, handle string, tools []map[string]any) error {
	body := map[string]any{"tools": tools}
	return s.client.do(ctx, "POST", fmt.Sprintf("/mcp/mcp_servers/%s/assign_tools", handle), body, nil)
}

// UpdateTool updates an assigned tool (e.g. its description) via
// PUT .../tools/:id.
func (s *mcpServerService) UpdateTool(ctx context.Context, handle string, toolID int, opts map[string]any) (*MCPServerTool, error) {
	var tool MCPServerTool
	if err := s.client.do(ctx, "PUT", fmt.Sprintf("/mcp/mcp_servers/%s/tools/%d", handle, toolID), opts, &tool); err != nil {
		return nil, err
	}
	return &tool, nil
}

// DeleteTool removes an assigned tool via DELETE .../tools/:id. Only tools
// backed by recipe functions or genies can be deleted.
func (s *mcpServerService) DeleteTool(ctx context.Context, handle string, toolID int) error {
	return s.client.do(ctx, "DELETE", fmt.Sprintf("/mcp/mcp_servers/%s/tools/%d", handle, toolID), nil, nil)
}

// GetServerPolicies reads the rate/quota/IP policy for a server.
func (s *mcpServerService) GetServerPolicies(ctx context.Context, handle string) (*MCPServerPolicy, error) {
	var policy MCPServerPolicy
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/mcp/mcp_servers/%s/server_policies", handle), nil, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

// SetServerPolicies updates the server policy. The API expects the policy
// fields nested under a "mcp_server_policy" key.
func (s *mcpServerService) SetServerPolicies(ctx context.Context, handle string, policy map[string]any) (*MCPServerPolicy, error) {
	body := map[string]any{"mcp_server_policy": policy}
	var updated MCPServerPolicy
	if err := s.client.do(ctx, "PUT", fmt.Sprintf("/mcp/mcp_servers/%s/server_policies", handle), body, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// AssignUserGroups grants the given IdP user groups access to an
// identity-based server via POST .../assign_user_groups.
func (s *mcpServerService) AssignUserGroups(ctx context.Context, handle string, groupIDs []string) error {
	body := map[string]any{"idp_user_group_ids": groupIDs}
	return s.client.do(ctx, "POST", fmt.Sprintf("/mcp/mcp_servers/%s/assign_user_groups", handle), body, nil)
}

// RemoveUserGroups revokes the given IdP user groups via
// POST .../remove_user_groups.
func (s *mcpServerService) RemoveUserGroups(ctx context.Context, handle string, groupIDs []string) error {
	body := map[string]any{"idp_user_group_ids": groupIDs}
	return s.client.do(ctx, "POST", fmt.Sprintf("/mcp/mcp_servers/%s/remove_user_groups", handle), body, nil)
}

// ListUserGroups lists IdP user groups (GET /api/mcp/user_groups), needed to
// resolve the group ids that assign/remove_user_groups consume.
func (s *mcpServerService) ListUserGroups(ctx context.Context, opts *PaginationOptions) ([]MCPUserGroup, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := "/mcp/user_groups"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var wrapper struct {
		Data []MCPUserGroup `json:"data"`
	}
	if err := s.client.do(ctx, "GET", path, nil, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data, nil
}
