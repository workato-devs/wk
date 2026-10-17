package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// skillJSON is a skill as list, get and create return it (captured
// 2026-10-16, trimmed). There is no recipe_id or description: the recipe is
// provider_id.
const skillJSON = `{"id":"skl-001","type":"skill_recipe","name":"my-skill","provider_id":100,
"provider_type":"Recipe","folder_id":10,"project_id":5,"folders":[{"id":10,"name":"Skills"}],
"running":true,"genies":[{"genie_id":"gin-1"}],"genies_count":1,
"trigger_description":"When the user asks…","applications":["jira","py_eval"],
"created_at":"2025-07-08T08:09:09.998-07:00","updated_at":"2025-07-08T08:55:15.439-07:00"}`

func checkSkill(t *testing.T, s Skill) {
	t.Helper()
	if s.ID != "skl-001" || s.Type != "skill_recipe" || s.ProviderID != 100 || s.FolderID != 10 || s.ProjectID != 5 || !s.Running {
		t.Errorf("skill = %+v", s)
	}
	if s.RecipeID() != 100 {
		t.Errorf("RecipeID() = %d, want provider_id when provider_type is Recipe", s.RecipeID())
	}
	if len(s.Genies) != 1 || s.Genies[0].GenieID != "gin-1" || len(s.Applications) != 2 || len(s.Folders) != 1 {
		t.Errorf("nested fields = %+v / %+v / %+v", s.Genies, s.Applications, s.Folders)
	}
	if s.CreatedAt.IsZero() {
		t.Error("created_at not decoded")
	}
}

func TestSkillService_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/agentic/skills" {
			t.Errorf("%s %s, want GET /agentic/skills", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[` + skillJSON + `],"total":179,"page":1,"per_page":50}`))
	}))
	defer srv.Close()

	skills, pg, err := NewHTTPClient(srv.URL, "test-token").Skills().List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	checkSkill(t, skills[0])
	if pg.Total != 179 || !pg.HasNext() {
		t.Errorf("page = %+v, want HasNext from total", pg)
	}
}

func TestSkillService_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/agentic/skills/skl-001" {
			t.Errorf("%s %s, want GET /agentic/skills/skl-001", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":` + skillJSON + `}`))
	}))
	defer srv.Close()

	s, err := NewHTTPClient(srv.URL, "test-token").Skills().Get(context.Background(), "skl-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkSkill(t, *s)
}

func TestSkillService_Create(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/agentic/skills" {
			t.Errorf("%s %s, want POST /agentic/skills", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["recipe_id"] != float64(100) {
			t.Errorf("recipe_id = %v, want 100", body["recipe_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":` + skillJSON + `}`))
	}))
	defer srv.Close()

	s, err := NewHTTPClient(srv.URL, "test-token").Skills().Create(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkSkill(t, *s)
}
