package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFolderService_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if pid := r.URL.Query().Get("parent_id"); pid != "10" {
			t.Errorf("parent_id = %q, want 10", pid)
		}
		w.Header().Set("Content-Type", "application/json")
		// Production expects raw array (no wrapper).
		json.NewEncoder(w).Encode([]Folder{{ID: 1, Name: "child"}})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	pid := 10
	folders, err := client.Folders().List(context.Background(), &pid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(folders) != 1 || folders[0].Name != "child" {
		t.Errorf("got %+v, want 1 folder named child", folders)
	}
}

func TestFolderService_Create(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "new-folder" {
			t.Errorf("name = %v, want new-folder", body["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Folder{ID: 5, Name: "new-folder"})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	folder, err := client.Folders().Create(context.Background(), "new-folder", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.ID != 5 {
		t.Errorf("ID = %d, want 5", folder.ID)
	}
}

func TestFolderService_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/folders/7" {
			t.Errorf("path = %s, want /folders/7", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	err := client.Folders().Delete(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestFolderService_DeleteProject pins the separate endpoint that
// projects (top-level, is_project=true) require. DELETE /folders/{id}
// does not work for projects; DeleteProject routes to /projects/{id}.
func TestFolderService_DeleteProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/projects/9" {
			t.Errorf("path = %s, want /projects/9 (project delete must not route through /folders/)", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	if err := client.Folders().DeleteProject(context.Background(), 9); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestFolder_DeserializesIsProjectAndProjectID ensures the list response
// captures is_project AND the distinct project_id — folders-delete
// routing depends on both: is_project picks the endpoint,
// project_id is the value passed to DELETE /projects/{project_id}
// (distinct from the folder's own id).
func TestFolder_DeserializesIsProjectAndProjectID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"name":"Proj","is_project":true,"project_id":42},{"id":2,"name":"Sub","is_project":false,"parent_id":1}]`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	folders, err := client.Folders().List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("folders len = %d, want 2", len(folders))
	}
	if !folders[0].IsProject {
		t.Errorf("folders[0].IsProject = false, want true")
	}
	if folders[0].ProjectID != 42 {
		t.Errorf("folders[0].ProjectID = %d, want 42 (distinct from folder id=1)", folders[0].ProjectID)
	}
	if folders[1].IsProject {
		t.Errorf("folders[1].IsProject = true, want false")
	}
	if folders[1].ProjectID != 0 {
		t.Errorf("folders[1].ProjectID = %d, want 0 (plain folder, not a project)", folders[1].ProjectID)
	}
}
