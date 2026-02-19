package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecipeService_ListJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/recipes/42/jobs" {
			t.Errorf("path = %s, want /recipes/42/jobs", r.URL.Path)
		}
		if s := r.URL.Query().Get("status"); s != "succeeded" {
			t.Errorf("status = %q, want succeeded", s)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ListResult[Job]{Items: []Job{{ID: 1, RecipeID: 42, Status: "succeeded"}}})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	jobs, err := client.Recipes().ListJobs(context.Background(), 42, &JobListOptions{Status: "succeeded"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Status != "succeeded" {
		t.Errorf("got %+v, want 1 succeeded job", jobs)
	}
}

func TestRecipeService_Copy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/recipes/42/copy" {
			t.Errorf("path = %s, want /recipes/42/copy", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["folder_id"] != float64(100) {
			t.Errorf("folder_id = %v, want 100", body["folder_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Recipe{ID: 99, Name: "copy", FolderID: 100})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	recipe, err := client.Recipes().Copy(context.Background(), 42, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recipe.ID != 99 {
		t.Errorf("ID = %d, want 99", recipe.ID)
	}
}

func TestRecipeService_Connect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/recipes/42/connect" {
			t.Errorf("path = %s, want /recipes/42/connect", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["adapter_name"] != "salesforce" {
			t.Errorf("adapter_name = %v, want salesforce", body["adapter_name"])
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.Recipes().Connect(context.Background(), 42, "salesforce", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
