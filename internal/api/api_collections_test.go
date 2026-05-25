package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func intPtr(v int) *int { return &v }

func TestAPICollectionService_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api_collections" {
			t.Errorf("path = %s, want /api_collections", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]APICollection{{ID: 1, Name: "v1", Version: "1.0", URL: "https://example.com/v1", ProjectID: intPtr(10)}})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	collections, err := client.APICollections().List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(collections) != 1 || collections[0].Name != "v1" {
		t.Errorf("got %+v, want 1 collection named v1", collections)
	}
	c := collections[0]
	if c.Version != "1.0" {
		t.Errorf("Version = %q, want %q", c.Version, "1.0")
	}
	if c.URL != "https://example.com/v1" {
		t.Errorf("URL = %q, want %q", c.URL, "https://example.com/v1")
	}
	if c.ProjectID == nil || *c.ProjectID != 10 {
		t.Errorf("ProjectID = %v, want 10", c.ProjectID)
	}
}

func TestAPICollectionService_Create(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "v2" {
			t.Errorf("name = %v, want v2", body["name"])
		}
		if body["project_id"] != float64(10) {
			t.Errorf("project_id = %v, want 10", body["project_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APICollection{ID: 2, Name: "v2", ProjectID: intPtr(10)})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	c, err := client.APICollections().Create(context.Background(), "v2", intPtr(10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != 2 {
		t.Errorf("ID = %d, want 2", c.ID)
	}
}

func TestAPICollectionService_CreateWithoutProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["project_id"]; ok {
			t.Error("project_id should not be sent when nil")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APICollection{ID: 3, Name: "no-project"})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	c, err := client.APICollections().Create(context.Background(), "no-project", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != 3 {
		t.Errorf("ID = %d, want 3", c.ID)
	}
	if c.ProjectID != nil {
		t.Errorf("ProjectID = %v, want nil", c.ProjectID)
	}
}
