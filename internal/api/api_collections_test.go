package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPICollectionService_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api_collections" {
			t.Errorf("path = %s, want /api_collections", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]APICollection{{ID: 1, Name: "v1", ProjectID: 10}})
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APICollection{ID: 2, Name: "v2", ProjectID: 10})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	c, err := client.APICollections().Create(context.Background(), "v2", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != 2 {
		t.Errorf("ID = %d, want 2", c.ID)
	}
}
