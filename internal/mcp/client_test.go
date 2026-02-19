package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Initialize_SSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Mcp-Session-Id", "test-session")
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"protocolVersion": "2025-03-26",
				"serverInfo": map[string]any{
					"name":    "test-server",
					"version": "1.0",
				},
				"capabilities": map[string]any{},
			},
		}
		data, _ := json.Marshal(resp)
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	info, err := client.Initialize(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "test-server" {
		t.Errorf("name = %q, want test-server", info.Name)
	}
	if info.ProtocolVersion != "2025-03-26" {
		t.Errorf("protocol = %q, want 2025-03-26", info.ProtocolVersion)
	}
	if client.sessionID != "test-session" {
		t.Errorf("sessionID = %q, want test-session", client.sessionID)
	}
}

func TestClient_Initialize_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"protocolVersion": "2025-03-26",
				"serverInfo": map[string]any{
					"name":    "json-server",
					"version": "2.0",
				},
				"capabilities": map[string]any{},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	info, err := client.Initialize(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "json-server" {
		t.Errorf("name = %q, want json-server", info.Name)
	}
}

func TestClient_ListTools(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")

		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method == "initialize" {
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"protocolVersion": "2025-03-26",
					"serverInfo":      map[string]any{"name": "test", "version": "1.0"},
					"capabilities":    map[string]any{},
				},
			}
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
		} else if req.Method == "tools/list" {
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"tools": []map[string]any{
						{
							"name":        "get_recipe",
							"description": "Get a recipe by ID",
							"inputSchema": map[string]any{"type": "object"},
						},
					},
				},
			}
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "get_recipe" {
		t.Errorf("got %+v, want 1 tool named get_recipe", tools)
	}
}

func TestClient_Initialize_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.Initialize(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}
