package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Shapes captured from a workspace with Agent Studio data and guardrails
// enabled (2026-10-16), trimmed.
var agenticFixtures = map[string]string{
	"/agentic/knowledge_bases": `{"data":[{"id":"kb-1","name":"Helpdesk","description":"d","source_type":"recipe",
"folder_id":63245,"project_id":42977,"folders":[{"id":63241,"name":"Lab"}],"external_id":"cd4e",
"active_recipes_count":0,"genies":[{"genie_id":"gin-1"}],"genies_count":1,"applications":[],
"created_at":"2026-08-21T17:49:59.290-07:00","updated_at":"2026-08-21T17:49:59.290-07:00"}],"total":37,"page":1,"per_page":20}`,
	"/agentic/knowledge_bases/kb-1":              `{"data":{"id":"kb-1","name":"Helpdesk","source_type":"recipe","storage_size":0}}`,
	"/agentic/knowledge_bases/kb-1/data_sources": `{"errors":[{"code":400,"title":"Data sources are only available for Matrix knowledge bases"}]}`,
	"/agentic/genies/clients": `{"data":[{"client_id":"gincl-1","client_name":"Headless","ip_allow_list":[],
"activated_at":"2026-10-06T14:53:50.350-07:00","genie_ids":["gin-1"],"status":"active","type":"api_key",
"base_host":"genie-api.trial.workato.com"}],"total":77,"page":1,"per_page":20}`,
	"/agentic/genies/gin-1/guardrails": `{"data":{"id":"aigrl-1","policies":[{"id":"aigrlp-1","policy_type":"harmful_content",
"configuration":{"filters":[{"type":"HATE","strength":"NONE"}]}},{"id":"aigrlp-2","policy_type":"some_future_type","configuration":{"x":1}}]}}`,
	"/agentic/genies/guardrails/policies/pii_entities": `{"data":["ADDRESS","AGE"]}`,
	"/agentic/genies/gin-1/conversations": `{"data":[{"id":"conversation-1","topic":"t","genie_handle":"gin-1","genie_id":"13215",
"genie_name":"PIER","genie_version":0,"app_type":"test_mode","chat_type":"GENIE","conversation_origin":"user",
"started_via":"Test mode message","started_by":{"user_id":"u1","name":"Ada","email":"ada@example.com"},
"test_conversation":true,"environment_id":2100000234,"environment_type":"dev","workspace_id":2100000234,
"created_at":1790825691596}],"next_cursor":"eyJhZnRlciI6MX0"}`,
	"/agentic/genies/gin-1/conversations/conversation-1": `{"data":{"id":"conversation-1","created_at":1790825691596,
"last_message_at":0,"totals":{"errors":{"count":1},"skill_calls":{"count":2},"kb_calls":{"count":3},
"feedback":{"positive":{"count":4,"comments":1},"negative":{"count":0,"comments":0}}}}}`,
	"/agentic/genies/gin-1/conversations/conversation-1/events": `{"data":[{"event_id":"e1","event_type":"genie_run_start",
"date_time":"2026-10-01T03:34:51.596Z","conversation_id":"conversation-1","genie_handle":"gin-1","genie_id":"13215",
"genie_run_id":"r1","identity_user_id":"u1","environment_id":1,"workspace_id":1,"app_type":"test_mode",
"first_run_in_conversation":true,"genie_run_trigger":"user","test_run":true},
{"event_id":"e2","event_type":"agent_message","date_time":"2026-10-01T03:34:56.38404Z","conversation_id":"conversation-1",
"genie_handle":"gin-1","genie_id":"13215","genie_run_id":"r1","identity_user_id":"u1","environment_id":1,"workspace_id":1,
"app_type":"test_mode","chat_type":"GENIE","request_id":"q1","data":"{\"message\":\"hi\"}"}],"next_cursor":null}`,
}

func agenticServer(t *testing.T, seen *[]string) *HTTPClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.RawQuery)
		body, ok := agenticFixtures[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(body, `{"errors"`) {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewHTTPClient(srv.URL, "tok")
}

func TestKnowledgeBaseService(t *testing.T) {
	var seen []string
	c := agenticServer(t, &seen)
	ctx := context.Background()

	kbs, pg, err := c.KnowledgeBases().List(ctx, &KBListOptions{Query: "help", PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(kbs) != 1 || kbs[0].GeniesCount != 1 || kbs[0].Genies[0].GenieID != "gin-1" || !pg.HasNext() {
		t.Errorf("list = %+v, page = %+v", kbs, pg)
	}
	if !strings.Contains(seen[0], "query=help") {
		t.Errorf("query = %s", seen[0])
	}
	// storage_size is get-only: list must not report a value it wasn't sent.
	if kbs[0].StorageSize != nil {
		t.Errorf("list StorageSize = %d, want unset", *kbs[0].StorageSize)
	}
	kb, err := c.KnowledgeBases().Get(ctx, "kb-1")
	if err != nil || kb.StorageSize == nil || *kb.StorageSize != 0 {
		t.Errorf("get = %+v, %v", kb, err)
	}
	// Agent Studio errors are {"errors":[{"title":…}]}; the title is the message.
	_, err = c.KnowledgeBases().DataSources(ctx, "kb-1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "Data sources are only available for Matrix knowledge bases" {
		t.Errorf("data sources err = %v", err)
	}
}

func TestGenieClientAndGuardrailServices(t *testing.T) {
	var seen []string
	c := agenticServer(t, &seen)
	ctx := context.Background()

	clients, pg, err := c.GenieClients().List(ctx, &PaginationOptions{Page: 2})
	if err != nil || len(clients) != 1 || clients[0].GenieIDs[0] != "gin-1" || clients[0].ActivatedAt.IsZero() || pg.Total != 77 {
		t.Errorf("clients = %+v, %+v, %v", clients, pg, err)
	}
	if !strings.Contains(seen[0], "page=2") {
		t.Errorf("query = %s", seen[0])
	}

	g, err := c.Guardrails().Get(ctx, "gin-1")
	if err != nil || g.ID == nil || len(g.Policies) != 2 {
		t.Fatalf("guardrail = %+v, %v", g, err)
	}
	// An unknown policy type still reads; its configuration is kept as sent.
	if g.Policies[1].PolicyType != "some_future_type" || string(g.Policies[1].Configuration) != `{"x":1}` {
		t.Errorf("policy = %+v", g.Policies[1])
	}
	entities, err := c.Guardrails().PIIEntities(ctx)
	if err != nil || len(entities) != 2 {
		t.Errorf("pii = %v, %v", entities, err)
	}
}

func TestConversationService(t *testing.T) {
	var seen []string
	c := agenticServer(t, &seen)
	ctx := context.Background()

	convs, cp, err := c.Conversations().List(ctx, "gin-1", &ConversationListOptions{Cursor: CursorOptions{After: "abc", Size: 5}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen[0], "page%5Bafter%5D=abc") || !strings.Contains(seen[0], "page%5Bsize%5D=5") {
		t.Errorf("query = %s, want page[after] and page[size]", seen[0])
	}
	if len(convs) != 1 || convs[0].StartedBy.Name != "Ada" || !cp.HasNext || cp.NextCursor != "eyJhZnRlciI6MX0" {
		t.Errorf("list = %+v, cursor = %+v", convs, cp)
	}
	if out, _ := json.Marshal(convs[0].CreatedAt); string(out) != `"2026-10-01T03:34:51.596Z"` {
		t.Errorf("created_at = %s, want epoch millis normalized to RFC 3339", out)
	}

	d, err := c.Conversations().Get(ctx, "gin-1", "conversation-1")
	if err != nil || d.Totals.SkillCalls.Count != 2 || d.Totals.Feedback.Positive.Comments != 1 {
		t.Errorf("detail = %+v, %v", d, err)
	}
	// last_message_at 0 means no messages, not 1970.
	if !d.LastMessageAt.IsZero() {
		t.Errorf("last_message_at = %s, want unset", d.LastMessageAt)
	}

	events, ep, err := c.Conversations().Events(ctx, "gin-1", "conversation-1", &EventListOptions{EventTypes: []string{"agent_message", "user_message"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen[2], "include_event_types%5B%5D=agent_message") || strings.Count(seen[2], "include_event_types") != 2 {
		t.Errorf("query = %s, want repeated include_event_types[]", seen[2])
	}
	// next_cursor null: no next page.
	if ep.HasNext || len(events) != 2 {
		t.Errorf("events = %d, cursor = %+v", len(events), ep)
	}
	if events[0].FirstRunInConversation == nil || !*events[0].FirstRunInConversation || events[0].Data != nil {
		t.Errorf("run start = %+v", events[0])
	}
	if events[1].Data == nil || *events[1].Data != `{"message":"hi"}` || events[1].RequestID != "q1" {
		t.Errorf("agent message = %+v", events[1])
	}
}

func TestKnowledgeBaseService_Writes(t *testing.T) {
	type call struct{ method, path, body string }
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, call{r.Method, r.URL.Path, string(b)})
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"data":{"id":"kb-new","name":"N","source_type":"matrix","folder_id":7,"genies":[]}}`))
			return
		}
		w.Write([]byte(`{"data":null}`))
	}))
	defer srv.Close()
	kbs := NewHTTPClient(srv.URL, "tok").KnowledgeBases()
	ctx := context.Background()
	name, folder, src := "N", "7", "matrix"

	// Create returns the whole object, unlike genie create.
	kb, err := kbs.Create(ctx, KBInput{Name: &name, FolderID: &folder, SourceType: &src})
	if err != nil || kb.ID != "kb-new" || kb.SourceType != "matrix" || kb.FolderID != 7 {
		t.Fatalf("create = %+v, %v", kb, err)
	}
	if err := kbs.Update(ctx, "kb-1", KBInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if err := kbs.Delete(ctx, "kb-1"); err != nil {
		t.Fatal(err)
	}
	want := []call{
		{"POST", "/agentic/knowledge_bases", `{"name":"N","folder_id":"7","source_type":"matrix"}`},
		{"PUT", "/agentic/knowledge_bases/kb-1", `{"name":"N"}`},
		{"DELETE", "/agentic/knowledge_bases/kb-1", ""},
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
