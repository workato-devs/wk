package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIEndpointService_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if cid := r.URL.Query().Get("api_collection_id"); cid != "5" {
			t.Errorf("api_collection_id = %q, want 5", cid)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"name":"ep1","api_collection_id":5,"active":true,"method":"GET","path":"/users","flow_id":42,"description":"test desc"}]`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	cid := 5
	endpoints, err := client.APIEndpoints().List(context.Background(), &cid, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 1 || !endpoints[0].Active {
		t.Errorf("got %+v, want 1 active endpoint", endpoints)
	}
	ep := endpoints[0]
	if ep.Method != "GET" {
		t.Errorf("Method = %q, want %q", ep.Method, "GET")
	}
	if ep.Path != "/users" {
		t.Errorf("Path = %q, want %q", ep.Path, "/users")
	}
	if ep.FlowID != 42 {
		t.Errorf("FlowID = %d, want 42", ep.FlowID)
	}
	if ep.RecipeID != 42 {
		t.Errorf("RecipeID = %d, want 42 (backfilled from FlowID)", ep.RecipeID)
	}
	if ep.Description == nil || *ep.Description != "test desc" {
		t.Errorf("Description = %v, want %q", ep.Description, "test desc")
	}
}

func TestAPIEndpointService_Create(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api_collections/5/api_endpoints" {
			t.Errorf("path = %s, want /api_collections/5/api_endpoints", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "Create User" {
			t.Errorf("name = %v, want Create User", body["name"])
		}
		if body["flow_id"] != float64(42) {
			t.Errorf("flow_id = %v, want 42", body["flow_id"])
		}
		if body["description"] != "Creates a user" {
			t.Errorf("description = %v, want Creates a user", body["description"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":99,"name":"Create User","api_collection_id":5,"active":false,"method":"POST","path":"/users","flow_id":42,"description":"Creates a user"}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	data := []byte(`{"name":"Create User","method":"POST","path":"/users","flow_id":42,"description":"Creates a user"}`)
	ep, err := client.APIEndpoints().Create(context.Background(), 5, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.ID != 99 {
		t.Errorf("ID = %d, want 99", ep.ID)
	}
	if ep.RecipeID != 42 {
		t.Errorf("RecipeID = %d, want 42 (backfilled from flow_id)", ep.RecipeID)
	}
	if ep.Method != "POST" || ep.Path != "/users" {
		t.Errorf("Method/Path = %s %s, want POST /users", ep.Method, ep.Path)
	}
}

func TestAPIEndpointService_Enable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api_endpoints/3/enable" {
			t.Errorf("path = %s, want /api_endpoints/3/enable", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.APIEndpoints().Enable(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIEndpointService_Disable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api_endpoints/3/disable" {
			t.Errorf("path = %s, want /api_endpoints/3/disable", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.APIEndpoints().Disable(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
