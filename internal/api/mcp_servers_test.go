package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPServerService_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers" {
			t.Errorf("path = %s, want /mcp/mcp_servers", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"mcps-abc","name":"test-server","authentication_method":"token","tools_count":5,"folder_id":100,"project_id":42}],"count":1,"page":1,"per_page":20}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	servers, err := client.MCPServers().List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	if servers[0].ID != "mcps-abc" {
		t.Errorf("ID = %q, want %q", servers[0].ID, "mcps-abc")
	}
	if servers[0].Name != "test-server" {
		t.Errorf("Name = %q, want %q", servers[0].Name, "test-server")
	}
	if servers[0].ToolsCount != 5 {
		t.Errorf("ToolsCount = %d, want 5", servers[0].ToolsCount)
	}
}

func TestMCPServerService_ListWithFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("project_id") != "42" {
			t.Errorf("project_id = %q, want 42", r.URL.Query().Get("project_id"))
		}
		if r.URL.Query().Get("authentication_method") != "token" {
			t.Errorf("authentication_method = %q, want token", r.URL.Query().Get("authentication_method"))
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("page = %q, want 2", r.URL.Query().Get("page"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[],"count":0,"page":2,"per_page":20}`))
	}))
	defer srv.Close()

	projectID := 42
	client := NewHTTPClient(srv.URL, "test-token")
	servers, err := client.MCPServers().List(context.Background(), &MCPServerListOptions{
		ProjectID:            &projectID,
		AuthenticationMethod: "token",
		Page:                 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("got %d servers, want 0", len(servers))
	}
}

func TestMCPServerService_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers/mcps-abc" {
			t.Errorf("path = %s, want /mcp/mcp_servers/mcps-abc", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":"mcps-abc","name":"test-server","auth_type":"token","mcp_url":"https://example.com/mcp","tools_count":5,"folder_id":100,"project_id":42,"api_collection":{"id":10,"type":"recipe","name":"my-collection","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"},"folders":[{"id":100,"name":"my-folder"}],"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	s, err := client.MCPServers().Get(context.Background(), "mcps-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "mcps-abc" {
		t.Errorf("ID = %q, want %q", s.ID, "mcps-abc")
	}
	if s.MCPURL != "https://example.com/mcp" {
		t.Errorf("MCPURL = %q, want %q", s.MCPURL, "https://example.com/mcp")
	}
	if s.APICollection == nil || s.APICollection.Name != "my-collection" {
		t.Errorf("APICollection = %+v, want collection named my-collection", s.APICollection)
	}
	if len(s.Folders) != 1 || s.Folders[0].Name != "my-folder" {
		t.Errorf("Folders = %+v, want 1 folder named my-folder", s.Folders)
	}
}

func TestMCPServerService_Create(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers" {
			t.Errorf("path = %s, want /mcp/mcp_servers", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "new-server" {
			t.Errorf("name = %v, want new-server", body["name"])
		}
		if body["folder_id"] != float64(100) {
			t.Errorf("folder_id = %v, want 100", body["folder_id"])
		}
		if body["description"] != "test desc" {
			t.Errorf("description = %v, want test desc", body["description"])
		}
		if body["asset_id"] != float64(50) {
			t.Errorf("asset_id = %v, want 50", body["asset_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":"mcps-new","name":"new-server","mcp_url":"https://example.com/mcp/new","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	assetID := 50
	s, err := client.MCPServers().Create(context.Background(), "new-server", 100, "test desc", &assetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "mcps-new" {
		t.Errorf("ID = %q, want %q", s.ID, "mcps-new")
	}
	if s.MCPURL != "https://example.com/mcp/new" {
		t.Errorf("MCPURL = %q, want %q", s.MCPURL, "https://example.com/mcp/new")
	}
}

func TestMCPServerService_Update(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers/mcps-abc" {
			t.Errorf("path = %s, want /mcp/mcp_servers/mcps-abc", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "renamed" {
			t.Errorf("name = %v, want renamed", body["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":"mcps-abc","name":"renamed","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	s, err := client.MCPServers().Update(context.Background(), "mcps-abc", map[string]any{"name": "renamed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "renamed" {
		t.Errorf("Name = %q, want %q", s.Name, "renamed")
	}
}

func TestMCPServerService_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers/mcps-abc" {
			t.Errorf("path = %s, want /mcp/mcp_servers/mcps-abc", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.MCPServers().Delete(context.Background(), "mcps-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMCPServerService_TokenRenew(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers/mcps-abc/token_renew" {
			t.Errorf("path = %s, want /mcp/mcp_servers/mcps-abc/token_renew", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":"mcps-abc","name":"test-server","mcp_url":"https://example.com/mcp?wkt_token=new-token","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	s, err := client.MCPServers().TokenRenew(context.Background(), "mcps-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.MCPURL != "https://example.com/mcp?wkt_token=new-token" {
		t.Errorf("MCPURL = %q, want new-token URL", s.MCPURL)
	}
}

func TestMCPServerService_ListTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers/mcps-abc/tools" {
			t.Errorf("path = %s, want /mcp/mcp_servers/mcps-abc/tools", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		desc := "A test tool"
		_ = desc
		w.Write([]byte(`{"items":[{"id":1,"name":"test-tool","description":"A test tool","flow_id":42,"active":true,"enabled":true,"vua_required":false}],"count":1,"page":1,"per_page":100}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	tools, err := client.MCPServers().ListTools(context.Background(), "mcps-abc", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}
	if tools[0].Name != "test-tool" {
		t.Errorf("Name = %q, want %q", tools[0].Name, "test-tool")
	}
	if tools[0].FlowID != 42 {
		t.Errorf("FlowID = %d, want 42", tools[0].FlowID)
	}
	if !tools[0].Active {
		t.Error("Active = false, want true")
	}
}

func TestMCPServerService_AssignTools(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers/mcps-x/assign_tools" {
			t.Errorf("path = %s, want .../assign_tools", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	tools := []map[string]any{{"trigger_application": "workato_api_platform", "id": "277601"}}
	if err := client.MCPServers().AssignTools(context.Background(), "mcps-x", tools); err != nil {
		t.Fatalf("AssignTools: %v", err)
	}
	got, ok := captured["tools"].([]any)
	if !ok || len(got) != 1 {
		t.Fatalf("tools = %v, want 1-element array", captured["tools"])
	}
}

func TestMCPServerService_DeleteTool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers/mcps-x/tools/5" {
			t.Errorf("path = %s, want .../tools/5", r.URL.Path)
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	if err := client.MCPServers().DeleteTool(context.Background(), "mcps-x", 5); err != nil {
		t.Fatalf("DeleteTool: %v", err)
	}
}

func TestMCPServerService_GetServerPolicies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/mcp/mcp_servers/mcps-x/server_policies" {
			t.Errorf("got %s %s, want GET .../server_policies", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		// mcp_server_id is a string in the real API (regression: it was
		// decoded as int and always failed).
		w.Write([]byte(`{"id":501,"mcp_server_id":"mcps-x","rate_limits":{"limit":60,"interval":"minute"},"ip_allow_list":["203.0.113.0/24"]}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	policy, err := client.MCPServers().GetServerPolicies(context.Background(), "mcps-x")
	if err != nil {
		t.Fatalf("GetServerPolicies: %v", err)
	}
	if policy.MCPServerID == nil || *policy.MCPServerID != "mcps-x" {
		t.Errorf("mcp_server_id = %v, want mcps-x", policy.MCPServerID)
	}
	if policy.RateLimits["interval"] != "minute" {
		t.Errorf("rate interval = %v, want minute", policy.RateLimits["interval"])
	}
}

func TestMCPServerService_SetServerPolicies(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/mcp/mcp_servers/mcps-x/server_policies" {
			t.Errorf("path = %s, want .../server_policies", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		// The API returns mcp_server_id as a string and limits as
		// {limit, interval} objects.
		w.Write([]byte(`{"id":1,"mcp_server_id":"mcps-x","rate_limits":{"limit":30,"interval":"minute"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	policy := map[string]any{"rate_limits": map[string]any{"limit": 30, "interval": "minute"}}
	got, err := client.MCPServers().SetServerPolicies(context.Background(), "mcps-x", policy)
	if err != nil {
		t.Fatalf("SetServerPolicies: %v", err)
	}
	// Policy must be nested under mcp_server_policy in the request.
	if _, ok := captured["mcp_server_policy"].(map[string]any); !ok {
		t.Errorf("request body missing mcp_server_policy wrapper; got %v", captured)
	}
	if got.RateLimits["limit"] != float64(30) {
		t.Errorf("rate limit = %v, want 30", got.RateLimits["limit"])
	}
}

func TestMCPServerService_AssignUserGroups(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp/mcp_servers/mcps-x/assign_user_groups" {
			t.Errorf("path = %s, want .../assign_user_groups", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	if err := client.MCPServers().AssignUserGroups(context.Background(), "mcps-x", []string{"group-abc123"}); err != nil {
		t.Fatalf("AssignUserGroups: %v", err)
	}
	ids, ok := captured["idp_user_group_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "group-abc123" {
		t.Errorf("idp_user_group_ids = %v, want [group-abc123]", captured["idp_user_group_ids"])
	}
}

func TestMCPServerService_ListUserGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp/user_groups" {
			t.Errorf("path = %s, want /mcp/user_groups", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"group-abc123","name":"Sales","users_count":24}],"count":1}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	groups, err := client.MCPServers().ListUserGroups(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListUserGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != "group-abc123" || groups[0].UsersCount != 24 {
		t.Errorf("groups = %+v, want 1 group group-abc123 with 24 users", groups)
	}
}
