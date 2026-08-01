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

func TestFolderService_Update(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/folders/7" {
			t.Errorf("path = %s, want /folders/7", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "renamed" {
			t.Errorf("name = %v, want renamed", body["name"])
		}
		if _, ok := body["parent_id"]; ok {
			t.Errorf("parent_id = %v, want absent (nil parentID must be omitted)", body["parent_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Folder{ID: 7, Name: "renamed"})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	name := "renamed"
	folder, err := client.Folders().Update(context.Background(), 7, &name, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.Name != "renamed" {
		t.Errorf("name = %q, want renamed", folder.Name)
	}
}

// TestFolderService_Update_Reparent pins that Update can move a folder via
// parent_id alone (no name), and that a nil name is omitted from the request
// body rather than sent as an empty string that would blank out the folder's
// name server-side.
func TestFolderService_Update_Reparent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/folders/7" {
			t.Errorf("path = %s, want /folders/7", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["name"]; ok {
			t.Errorf("name = %v, want absent (nil name must be omitted)", body["name"])
		}
		if body["parent_id"] != float64(456) {
			t.Errorf("parent_id = %v, want 456", body["parent_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		parentID := 456
		json.NewEncoder(w).Encode(Folder{ID: 7, Name: "unchanged", ParentID: &parentID})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	parentID := 456
	folder, err := client.Folders().Update(context.Background(), 7, nil, &parentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.ParentID == nil || *folder.ParentID != 456 {
		t.Errorf("parentID = %v, want 456", folder.ParentID)
	}
}

// TestFolderService_Update_RenameAndReparent pins that a single Update call
// can send both fields at once, since PUT /folders/{id} is one endpoint for
// both operations rather than needing a separate Move wrapper.
func TestFolderService_Update_RenameAndReparent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "renamed" {
			t.Errorf("name = %v, want renamed", body["name"])
		}
		if body["parent_id"] != float64(456) {
			t.Errorf("parent_id = %v, want 456", body["parent_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		parentID := 456
		json.NewEncoder(w).Encode(Folder{ID: 7, Name: "renamed", ParentID: &parentID})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	name := "renamed"
	parentID := 456
	folder, err := client.Folders().Update(context.Background(), 7, &name, &parentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.Name != "renamed" || folder.ParentID == nil || *folder.ParentID != 456 {
		t.Errorf("got name=%q parentID=%v, want name=renamed parentID=456", folder.Name, folder.ParentID)
	}
}

// TestFolderService_UpdateProject pins the separate PUT /projects/{id}
// endpoint that projects (is_project=true) require — mirroring
// DeleteProject. A project update must not route through /folders/.
func TestFolderService_UpdateProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/projects/9" {
			t.Errorf("path = %s, want /projects/9 (project update must not route through /folders/)", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Folder{ID: 3, Name: "renamed", IsProject: true, ProjectID: 9})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	folder, err := client.Folders().UpdateProject(context.Background(), 9, "renamed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.Name != "renamed" {
		t.Errorf("name = %q, want renamed", folder.Name)
	}
}

func TestFolderService_ListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/projects" {
			t.Errorf("path = %s, want /projects", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Folder{{ID: 1, Name: "Proj", IsProject: true, ProjectID: 42}})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	projects, err := client.Folders().ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Proj" {
		t.Errorf("got %+v, want 1 project named Proj", projects)
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
