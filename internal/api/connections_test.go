package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectionService_Create(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "my-conn" {
			t.Errorf("name = %v, want my-conn", body["name"])
		}
		if body["provider"] != "salesforce" {
			t.Errorf("provider = %v, want salesforce", body["provider"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Connection{ID: 1, Name: "my-conn", Application: "salesforce"})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	conn, err := client.Connections().Create(context.Background(), "my-conn", "salesforce", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.ID != 1 {
		t.Errorf("ID = %d, want 1", conn.ID)
	}
}

func TestConnectionService_Update(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/connections/5" {
			t.Errorf("path = %s, want /connections/5", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Connection{ID: 5, Name: "updated"})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	conn, err := client.Connections().Update(context.Background(), 5, "updated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Name != "updated" {
		t.Errorf("name = %q, want updated", conn.Name)
	}
}

func TestConnectionService_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/connections/5" {
			t.Errorf("path = %s, want /connections/5", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.Connections().Delete(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnectionService_Disconnect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/connections/5/disconnect" {
			t.Errorf("path = %s, want /connections/5/disconnect", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.Connections().Disconnect(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
