package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Shapes captured from GET /agentic/genies and /agentic/genies/:id
// (2026-10-16), trimmed.
const genieListJSON = `{"data":[{"id":"gin-1","name":"Concierge","description":"d","state":"stopped",
"folder_id":42,"instructions":"Be helpful.","ai_provider":"anthropic","ai_model":"claude-sonnet-4-6",
"chat_interface":"api","channel_mode":"dms_only","channel_scope":"specific","channel_whitelist":[],
"response_mode":"on_mention","web_search_enabled":false,"web_search_available":false,
"code_interpreter_enabled":false,"skills_count":2,"knowledge_bases_count":1,"user_groups_count":0,
"active_recipes_count":0,"inactive_skills_count":0,"triggers":[],"logo_url":"https://x/logo.png",
"created_at":"2026-10-16T14:31:31.565-07:00","updated_at":"2026-10-16T14:31:31.565-07:00"}],
"total":51,"page":1,"per_page":50}`

const genieGetJSON = `{"data":{"id":"gin-1","name":"Concierge","state":"inactive","folder_id":42,
"ai_model":"claude-sonnet-4-6","ai_model_label":"Claude Sonnet 4.6","ai_provider_id":"claude-sonnet-4-6",
"ai_provider_type":"default","connection_id":null,"custom_oauth_key_id":null,"project_id":7,
"folders":[{"id":42,"name":"Agents"}],"user_groups":[{"id":"6KDi","name":"Lab"}],
"created_at":"2026-10-16T14:31:31.565-07:00"}}`

func TestGenieService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/agentic/genies":
			if r.URL.Query().Get("state") != "stopped" || r.URL.Query().Get("per_page") != "50" {
				t.Errorf("query = %s", r.URL.RawQuery)
			}
			w.Write([]byte(genieListJSON))
		case "/agentic/genies/gin-1":
			w.Write([]byte(genieGetJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := NewHTTPClient(srv.URL, "tok")

	genies, pg, err := c.Genies().List(context.Background(), &GenieListOptions{State: "stopped", Page: 1, PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(genies) != 1 || genies[0].State != "stopped" || genies[0].SkillsCount != 2 || *genies[0].ChatInterface != "api" {
		t.Errorf("genie = %+v", genies[0])
	}
	// total 51 at 50 per page: there is a next page, though this one isn't "full".
	if !pg.HasNext() || pg.Total != 51 {
		t.Errorf("page = %+v, HasNext = %v; want a next page from total", pg, pg.HasNext())
	}
	if want := time.Date(2026, 10, 16, 21, 31, 31, 565000000, time.UTC); !genies[0].CreatedAt.Equal(want) {
		t.Errorf("created_at = %s, want %s", genies[0].CreatedAt, want)
	}

	g, err := c.Genies().Get(context.Background(), "gin-1")
	if err != nil {
		t.Fatal(err)
	}
	// The detail endpoint's state vocabulary is kept, not translated.
	if g.State != "inactive" || g.ProjectID != 7 || g.AIModelLabel != "Claude Sonnet 4.6" ||
		len(g.Folders) != 1 || g.UserGroups[0].ID != "6KDi" || string(g.ConnectionID) != "null" {
		t.Errorf("detail = %+v", g)
	}
}

func TestOffsetPage_HasNext(t *testing.T) {
	for _, tt := range []struct {
		p    OffsetPage
		want bool
	}{
		{OffsetPage{Page: 1, PerPage: 20, Total: 20}, false},
		{OffsetPage{Page: 1, PerPage: 20, Total: 21}, true},
		{OffsetPage{Page: 2, PerPage: 20, Total: 40}, false},
		{OffsetPage{}, false},
	} {
		if got := tt.p.HasNext(); got != tt.want {
			t.Errorf("%+v HasNext = %v, want %v", tt.p, got, tt.want)
		}
	}
}

func TestGenieService_Writes(t *testing.T) {
	type call struct{ method, path, body string }
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, call{r.Method, r.URL.Path, string(b)})
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && r.URL.Path == "/agentic/genies" {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"data":{"id":"gin-new"}}`))
			return
		}
		w.Write([]byte(`{"data":null}`))
	}))
	defer srv.Close()
	g := NewHTTPClient(srv.URL, "tok").Genies()
	ctx := context.Background()
	name := "N"

	id, err := g.Create(ctx, GenieInput{Name: &name})
	if err != nil || id != "gin-new" {
		t.Fatalf("create = %q, %v", id, err)
	}
	g.Update(ctx, "gin-1", GenieInput{Name: &name})
	g.Start(ctx, "gin-1")
	g.Stop(ctx, "gin-1")
	g.Assign(ctx, "gin-1", AssignKnowledgeBases, []string{"kb-1"})
	g.Remove(ctx, "gin-1", AssignSkills, []string{"skl-1"})
	g.Assign(ctx, "gin-1", AssignUserGroups, []string{"ug-1"})
	g.Delete(ctx, "gin-1")

	want := []call{
		{"POST", "/agentic/genies", `{"name":"N"}`},
		{"PUT", "/agentic/genies/gin-1", `{"name":"N"}`},
		{"POST", "/agentic/genies/gin-1/start", ""},
		{"POST", "/agentic/genies/gin-1/stop", ""},
		{"POST", "/agentic/genies/gin-1/assign_knowledge_bases", `{"knowledge_base_ids":["kb-1"]}`},
		{"POST", "/agentic/genies/gin-1/remove_skills", `{"skill_ids":["skl-1"]}`},
		{"POST", "/agentic/genies/gin-1/assign_user_groups", `{"user_group_ids":["ug-1"]}`},
		{"DELETE", "/agentic/genies/gin-1", ""},
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %+v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d = %+v, want %+v", i, calls[i], want[i])
		}
	}
}
