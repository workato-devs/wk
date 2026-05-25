package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIClientService_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v2/api_clients" {
			t.Errorf("path = %s, want /v2/api_clients", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":1,"name":"test-client","auth_type":"token","active_api_keys_count":2}],"count":1,"page":1,"per_page":100}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	clients, err := client.APIClients().List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("got %d clients, want 1", len(clients))
	}
	if clients[0].Name != "test-client" {
		t.Errorf("Name = %q, want %q", clients[0].Name, "test-client")
	}
	if clients[0].AuthType != "token" {
		t.Errorf("AuthType = %q, want %q", clients[0].AuthType, "token")
	}
	if clients[0].ActiveAPIKeysCount != 2 {
		t.Errorf("ActiveAPIKeysCount = %d, want 2", clients[0].ActiveAPIKeysCount)
	}
}

func TestAPIClientService_ListWithPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("page = %q, want 2", r.URL.Query().Get("page"))
		}
		if r.URL.Query().Get("per_page") != "10" {
			t.Errorf("per_page = %q, want 10", r.URL.Query().Get("per_page"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[],"count":0,"page":2,"per_page":10}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	clients, err := client.APIClients().List(context.Background(), &PaginationOptions{Page: 2, PerPage: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(clients) != 0 {
		t.Errorf("got %d clients, want 0", len(clients))
	}
}

func TestAPIClientService_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v2/api_clients/42" {
			t.Errorf("path = %s, want /v2/api_clients/42", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":42,"name":"prod-client","auth_type":"token","api_collections":[{"id":1,"name":"v1"}],"api_keys":[{"id":10,"name":"key-1","active":true}]}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	ac, err := client.APIClients().Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ac.ID != 42 {
		t.Errorf("ID = %d, want 42", ac.ID)
	}
	if len(ac.APICollections) != 1 || ac.APICollections[0].Name != "v1" {
		t.Errorf("APICollections = %+v, want 1 collection named v1", ac.APICollections)
	}
	if len(ac.APIKeys) != 1 || ac.APIKeys[0].Name != "key-1" {
		t.Errorf("APIKeys = %+v, want 1 key named key-1", ac.APIKeys)
	}
}

func TestAPIClientService_Create(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v2/api_clients" {
			t.Errorf("path = %s, want /v2/api_clients", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "new-client" {
			t.Errorf("name = %v, want new-client", body["name"])
		}
		if body["auth_type"] != "token" {
			t.Errorf("auth_type = %v, want token", body["auth_type"])
		}
		ids, ok := body["api_collection_ids"].([]any)
		if !ok || len(ids) != 2 {
			t.Errorf("api_collection_ids = %v, want [\"1\",\"2\"]", body["api_collection_ids"])
		} else {
			if ids[0] != "1" || ids[1] != "2" {
				t.Errorf("api_collection_ids = %v, want [\"1\",\"2\"]", ids)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":99,"name":"new-client","auth_type":"token"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	ac, err := client.APIClients().Create(context.Background(), "new-client", []string{"1", "2"}, "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ac.ID != 99 {
		t.Errorf("ID = %d, want 99", ac.ID)
	}
}

func TestAPIClientService_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/v2/api_clients/42" {
			t.Errorf("path = %s, want /v2/api_clients/42", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.APIClients().Delete(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIClientService_CreateKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v2/api_clients/42/api_keys" {
			t.Errorf("path = %s, want /v2/api_clients/42/api_keys", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "prod-key" {
			t.Errorf("name = %v, want prod-key", body["name"])
		}
		if body["active"] != true {
			t.Errorf("active = %v, want true", body["active"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":10,"name":"prod-key","auth_type":"token","auth_token":"secret-abc-123","active":true}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	key, err := client.APIClients().CreateKey(context.Background(), 42, "prod-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.ID != 10 {
		t.Errorf("ID = %d, want 10", key.ID)
	}
	if key.AuthToken != "secret-abc-123" {
		t.Errorf("AuthToken = %q, want %q", key.AuthToken, "secret-abc-123")
	}
	if !key.Active {
		t.Error("Active = false, want true")
	}
}

func TestAPIClientService_RefreshKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/v2/api_clients/42/api_keys/10/refresh_secret" {
			t.Errorf("path = %s, want /v2/api_clients/42/api_keys/10/refresh_secret", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":10,"name":"prod-key","auth_type":"token","auth_token":"new-secret-456","active":true}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	key, err := client.APIClients().RefreshKey(context.Background(), 42, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.AuthToken != "new-secret-456" {
		t.Errorf("AuthToken = %q, want %q", key.AuthToken, "new-secret-456")
	}
}
