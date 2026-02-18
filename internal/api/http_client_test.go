package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientDo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Recipe{ID: 1, Name: "test"})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	var recipe Recipe
	err := client.do(context.Background(), "GET", "/recipes/1", nil, &recipe)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recipe.ID != 1 || recipe.Name != "test" {
		t.Errorf("got %+v, want ID=1 Name=test", recipe)
	}
}

func TestHTTPClientDo_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "bad-token")
	err := client.do(context.Background(), "GET", "/recipes", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if !apiErr.IsUnauthorized() {
		t.Errorf("expected 401, got %d", apiErr.StatusCode)
	}
}

func TestHTTPClientDo_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.do(context.Background(), "GET", "/recipes/999", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Errorf("expected 404, got %d", apiErr.StatusCode)
	}
}

func TestHTTPClientDo_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"message":"Internal server error"}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.do(context.Background(), "GET", "/recipes", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("expected 500, got %d", apiErr.StatusCode)
	}
}

func TestPackageImport_RestartRecipesParam(t *testing.T) {
	tests := []struct {
		name            string
		restartRecipes  bool
		wantQueryParam  string
	}{
		{"restart_recipes=true", true, "true"},
		{"restart_recipes=false", false, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got := r.URL.Query().Get("restart_recipes")
				if got != tt.wantQueryParam {
					t.Errorf("restart_recipes = %q, want %q", got, tt.wantQueryParam)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"id":99}`))
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.URL, "test-token")
			svc := &packageService{client: client}
			id, err := svc.Import(context.Background(), 42, []byte("zipdata"), tt.restartRecipes)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != 99 {
				t.Errorf("import ID = %d, want 99", id)
			}
		})
	}
}
