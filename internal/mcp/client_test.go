package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// initServer answers initialize and records every request's method and
// Authorization header.
func initServer(t *testing.T) (*httptest.Server, *[]string, *[]string) {
	t.Helper()
	var methods, auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			Method string `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&msg)
		methods = append(methods, msg.Method)
		auths = append(auths, r.Header.Get("Authorization"))
		if msg.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{
			"protocolVersion": "2024-11-05", "serverInfo": map[string]any{"name": "s", "version": "1"},
		}})
	}))
	t.Cleanup(srv.Close)
	return srv, &methods, &auths
}

// The MCP lifecycle requires notifications/initialized after initialize.
func TestClient_Initialize_SendsInitializedNotification(t *testing.T) {
	srv, methods, _ := initServer(t)
	if _, err := NewClient(srv.URL).Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(*methods) != 2 || (*methods)[0] != "initialize" || (*methods)[1] != "notifications/initialized" {
		t.Errorf("methods = %v, want initialize then notifications/initialized", *methods)
	}
}

func TestClient_WithBearer(t *testing.T) {
	srv, _, auths := initServer(t)
	if _, err := NewClient(srv.URL).WithBearer("tok123").Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i, a := range *auths {
		if a != "Bearer tok123" {
			t.Errorf("request %d Authorization = %q, want Bearer tok123", i, a)
		}
	}

	srv2, _, auths2 := initServer(t)
	NewClient(srv2.URL).Initialize(context.Background())
	if (*auths2)[0] != "" {
		t.Errorf("Authorization sent without WithBearer: %q", (*auths2)[0])
	}
}

// A 401 keeps the body and the WWW-Authenticate challenge, which say why.
func TestClient_HTTPErrorKeepsBodyAndChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="https://x/.well-known/oauth-protected-resource"`)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"reason":"authentication_failed"}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL).Initialize(context.Background())
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("err = %v, want an *HTTPError", err)
	}
	if httpErr.StatusCode != 401 || !strings.Contains(httpErr.Body, "authentication_failed") || !strings.Contains(httpErr.WWWAuthenticate, "resource_metadata") {
		t.Errorf("HTTPError = %+v", httpErr)
	}
}
