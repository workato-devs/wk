package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		json.NewEncoder(w).Encode(ListResult[Job]{Items: []Job{{ID: "j-1", RecipeID: 42, Status: "succeeded"}}})
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

func TestRecipeService_GetJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/recipes/42/jobs/j-abc123" {
			t.Errorf("path = %s, want /recipes/42/jobs/j-abc123", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := `{
			"id": "j-abc123", "recipe_id": 42, "status": "failed",
			"is_error": true, "error": "Connection timeout",
			"handle": "j-abc123", "job_correlation_id": "corr-xyz",
			"error_parts": {
				"message": "No automatic OAuth refresh available, needs user re-authorization.",
				"error_type": "Exception",
				"error_id": "35601e00-f6c6-4c41-940e-c2be62efcfef",
				"action": "make_request_v2",
				"line_number": 1,
				"adapter": "rest",
				"retry_count": 0
			},
			"lines": [
				{
					"recipe_line_number": 1, "adapter_name": "rest", "adapter_operation": "make_request_v2",
					"input": {"url": "https://example.com"},
					"line_stat": {
						"total": 0.0079,
						"details": [
							{"name": "http", "count": 1, "average": 0.0079, "total": 0.0079, "min": 0.0079, "max": 0.0079}
						]
					},
					"error": "No automatic OAuth refresh available, needs user re-authorization.",
					"error_descriptor": {
						"error_type": "Exception",
						"error_id": "35601e00-f6c6-4c41-940e-c2be62efcfef",
						"line_number": null,
						"adapter": "Workato",
						"error_at": "2026-07-01T05:17:23.187-07:00",
						"actionable": true,
						"action": null,
						"trigger": null
					},
					"error_details": {
						"http_response": {
							"code": 401,
							"raw_status_text": "Unauthorized",
							"body": "{\"code\":401,\"message\":\"Unauthorized\"}",
							"headers": {"x_failure_category": "FAILURE_CLIENT_AUTH"}
						}
					}
				},
				{"recipe_line_number": 2, "adapter_name": "logger", "adapter_operation": "log_message"}
			]
		}`
		w.Write([]byte(resp))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	detail, err := client.Recipes().GetJob(context.Background(), 42, "j-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Status != "failed" {
		t.Errorf("status = %q, want failed", detail.Status)
	}
	if detail.Error == nil || *detail.Error != "Connection timeout" {
		t.Errorf("error = %v, want 'Connection timeout'", detail.Error)
	}
	if detail.Handle != "j-abc123" {
		t.Errorf("handle = %q, want j-abc123", detail.Handle)
	}
	if len(detail.Lines) != 2 {
		t.Fatalf("lines count = %d, want 2", len(detail.Lines))
	}
	if detail.Lines[0].AdapterName != "rest" {
		t.Errorf("lines[0].adapter_name = %q, want rest", detail.Lines[0].AdapterName)
	}
	// Per-step diagnostics must survive unmarshalling (issue #89): the
	// step error and the downstream HTTP response are the whole point of
	// fetching a single job's detail.
	if detail.JobCorrelationID != "corr-xyz" {
		t.Errorf("job_correlation_id = %q, want corr-xyz", detail.JobCorrelationID)
	}
	line := detail.Lines[0]
	if line.Error == nil || *line.Error == "" {
		t.Fatalf("lines[0].error = %v, want non-empty step error", line.Error)
	}
	if line.ErrorDetails == nil || line.ErrorDetails.HTTPResponse == nil {
		t.Fatalf("lines[0].error_details.http_response is nil, want the captured 401")
	}
	if got := line.ErrorDetails.HTTPResponse.Code; got != 401 {
		t.Errorf("lines[0].error_details.http_response.code = %d, want 401", got)
	}
	if len(line.Input) == 0 {
		t.Errorf("lines[0].input was dropped, want raw JSON preserved")
	}
	// Timing decodes as fractional seconds (issue #89 regression): the API
	// returns line_stat.total as a float like 0.0079, and each detail is a
	// {count, average, total, min, max} metric set — an int-typed total or a
	// scalar "value" detail hard-fails the whole decode.
	if line.LineStat == nil || line.LineStat.Total == nil {
		t.Fatalf("lines[0].line_stat.total is nil, want fractional duration")
	}
	if got := *line.LineStat.Total; got != 0.0079 {
		t.Errorf("lines[0].line_stat.total = %v, want 0.0079", got)
	}
	if len(line.LineStat.Details) != 1 {
		t.Fatalf("lines[0].line_stat.details = %d, want 1", len(line.LineStat.Details))
	}
	d := line.LineStat.Details[0]
	if d.Name != "http" || d.Count == nil || *d.Count != 1 {
		t.Errorf("detail = {name:%q count:%v}, want {http 1}", d.Name, d.Count)
	}
	if d.Average == nil || d.Total == nil || d.Min == nil || d.Max == nil {
		t.Errorf("detail metrics dropped: avg:%v total:%v min:%v max:%v", d.Average, d.Total, d.Min, d.Max)
	}
	// error_descriptor and error_parts use the API's real field names
	// (error_type, error_id, adapter, actionable, action, trigger, line_number,
	// retry_count) — not "type"/"message", which don't appear in either object
	// and would silently decode both structs to their zero value.
	if line.ErrorDescriptor == nil {
		t.Fatalf("lines[0].error_descriptor is nil, want populated")
	}
	if line.ErrorDescriptor.ErrorType != "Exception" {
		t.Errorf("error_descriptor.error_type = %q, want Exception", line.ErrorDescriptor.ErrorType)
	}
	if line.ErrorDescriptor.ErrorID != "35601e00-f6c6-4c41-940e-c2be62efcfef" {
		t.Errorf("error_descriptor.error_id = %q, want 35601e00-f6c6-4c41-940e-c2be62efcfef", line.ErrorDescriptor.ErrorID)
	}
	if line.ErrorDescriptor.Adapter != "Workato" {
		t.Errorf("error_descriptor.adapter = %q, want Workato", line.ErrorDescriptor.Adapter)
	}
	if !line.ErrorDescriptor.Actionable {
		t.Errorf("error_descriptor.actionable = false, want true")
	}
	if line.ErrorDescriptor.ErrorAt == nil {
		t.Errorf("error_descriptor.error_at is nil, want parsed timestamp")
	}
	if detail.ErrorParts == nil {
		t.Fatalf("error_parts is nil, want populated")
	}
	if detail.ErrorParts.ErrorType != "Exception" {
		t.Errorf("error_parts.error_type = %q, want Exception", detail.ErrorParts.ErrorType)
	}
	if detail.ErrorParts.ErrorID != "35601e00-f6c6-4c41-940e-c2be62efcfef" {
		t.Errorf("error_parts.error_id = %q, want 35601e00-f6c6-4c41-940e-c2be62efcfef", detail.ErrorParts.ErrorID)
	}
	if detail.ErrorParts.Action != "make_request_v2" {
		t.Errorf("error_parts.action = %q, want make_request_v2", detail.ErrorParts.Action)
	}
	if detail.ErrorParts.LineNumber == nil || *detail.ErrorParts.LineNumber != 1 {
		t.Errorf("error_parts.line_number = %v, want 1", detail.ErrorParts.LineNumber)
	}
	if detail.ErrorParts.Adapter != "rest" {
		t.Errorf("error_parts.adapter = %q, want rest", detail.ErrorParts.Adapter)
	}
}

func TestRecipeService_Copy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/recipes/42/copy":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			// folder_id must be sent as a string; the API rejects a numeric value.
			if body["folder_id"] != "100" {
				t.Errorf("folder_id = %#v, want string \"100\"", body["folder_id"])
			}
			// The copy endpoint returns new_flow_id, not a recipe object.
			w.Write([]byte(`{"success":true,"new_flow_id":99}`))
		case r.Method == "GET" && r.URL.Path == "/recipes/99":
			json.NewEncoder(w).Encode(Recipe{ID: 99, Name: "copy", FolderID: 100})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	recipe, err := client.Recipes().Copy(context.Background(), 42, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Copy must return the populated new recipe (regression: it returned a
	// zero-value Recipe because the 201 body was decoded as a flat recipe).
	if recipe.ID != 99 {
		t.Errorf("ID = %d, want 99 (new_flow_id, fetched via Get)", recipe.ID)
	}
	if recipe.FolderID != 100 {
		t.Errorf("FolderID = %d, want 100", recipe.FolderID)
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

func TestRecipeService_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/recipes/42" {
			t.Errorf("path = %s, want /recipes/42", r.URL.Path)
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	if err := client.Recipes().Delete(context.Background(), 42); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestRecipeService_Import_FollowUpGet(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/recipes":
			json.NewDecoder(r.Body).Decode(&captured)
			w.Write([]byte(`{"success":true,"id":206448}`))
		case r.Method == "GET" && r.URL.Path == "/recipes/206448":
			json.NewEncoder(w).Encode(Recipe{ID: 206448, Name: "imported", FolderID: 14116})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	body := []byte(`{"name":"imported","code":{"x":1},"config":[{"k":"v"}]}`)

	client := NewHTTPClient(srv.URL, "test-token")
	recipe, err := client.Recipes().Import(context.Background(), 14116, body)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if recipe.ID != 206448 || recipe.Name != "imported" || recipe.FolderID != 14116 {
		t.Errorf("recipe = %+v, want ID=206448 Name=imported FolderID=14116", recipe)
	}
	if fid, ok := captured["folder_id"].(string); !ok || fid != "14116" {
		t.Errorf("folder_id = %v (%T), want string 14116", captured["folder_id"], captured["folder_id"])
	}
	if s, ok := captured["code"].(string); !ok || s != `{"x":1}` {
		t.Errorf("code not stringified; got %T %v", captured["code"], captured["code"])
	}
}

func TestRecipeService_Update_StringifiesCodeAndConfig(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/recipes/42" {
			t.Errorf("path = %s, want /recipes/42", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	// Export-style JSON: code and config are objects (not pre-stringified)
	// and a stray folder_id exists — Update must stringify the first two
	// and drop the third to match Import's contract.
	body := []byte(`{"name":"r","folder_id":99,"code":{"x":1},"config":[{"k":"v"}]}`)

	client := NewHTTPClient(srv.URL, "test-token")
	if err := client.Recipes().Update(context.Background(), 42, body); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, hasFolder := captured["folder_id"]; hasFolder {
		t.Errorf("folder_id should be stripped on update; body = %+v", captured)
	}
	if s, ok := captured["code"].(string); !ok || s != `{"x":1}` {
		t.Errorf("code not stringified; got %T %v", captured["code"], captured["code"])
	}
	if s, ok := captured["config"].(string); !ok || s != `[{"k":"v"}]` {
		t.Errorf("config not stringified; got %T %v", captured["config"], captured["config"])
	}
}

func TestCanonicalizeRecipeExport(t *testing.T) {
	// Raw GET /recipes/{id} shape: code is an escaped JSON string, the
	// version is "version_no", private/concurrency are absent, and runtime
	// fields (job counts) are present.
	raw := []byte(`{
		"id": 271230,
		"name": "my recipe",
		"description": "desc",
		"folder_id": 99,
		"code": "{\"number\":0,\"provider\":\"clock\"}",
		"config": [{"keyword":"trigger","provider":"clock","name":"clock"}],
		"version_no": 7,
		"job_succeeded_count": 5,
		"webhook_url": "https://example.com/hook"
	}`)

	out, err := CanonicalizeRecipeExport(raw)
	if err != nil {
		t.Fatalf("CanonicalizeRecipeExport: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}

	// code must be an object, not an escaped string (lint CODE_NOT_OBJECT).
	if _, ok := got["code"].(map[string]any); !ok {
		t.Errorf("code = %T, want object", got["code"])
	}
	// All required top-level keys present (lint MISSING_TOP_LEVEL_KEYS).
	for _, k := range []string{"version", "private", "concurrency", "name", "config"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing required key %q", k)
		}
	}
	// version_no maps to version.
	if v, ok := got["version"].(float64); !ok || v != 7 {
		t.Errorf("version = %v, want 7 (from version_no)", got["version"])
	}
	// platform defaults for fields the endpoint omits.
	if got["private"] != false {
		t.Errorf("private = %v, want false", got["private"])
	}
	if v, ok := got["concurrency"].(float64); !ok || v != 1 {
		t.Errorf("concurrency = %v, want 1", got["concurrency"])
	}
	// runtime-only fields are dropped.
	if _, ok := got["job_succeeded_count"]; ok {
		t.Error("job_succeeded_count should be dropped from canonical output")
	}
	if _, ok := got["webhook_url"]; ok {
		t.Error("webhook_url should be dropped from canonical output")
	}
}

func TestRecipeService_Move_SendsFolderID(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/recipes/42":
			// Raw export shape: code/config present, original folder_id.
			w.Write([]byte(`{"name":"r","folder_id":99,"code":{"x":1},"config":[{"k":"v"}]}`))
		case r.Method == "PUT" && r.URL.Path == "/recipes/42":
			json.NewDecoder(r.Body).Decode(&captured)
			w.Write([]byte(`{"success":true}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	if err := client.Recipes().Move(context.Background(), 42, 678); err != nil {
		t.Fatalf("Move: %v", err)
	}
	// Unlike Update, Move must send the new folder_id (as a string, matching
	// Import's convention).
	if fid, ok := captured["folder_id"].(string); !ok || fid != "678" {
		t.Errorf("folder_id = %v (%T), want string 678", captured["folder_id"], captured["folder_id"])
	}
	if s, ok := captured["code"].(string); !ok || s != `{"x":1}` {
		t.Errorf("code not stringified; got %T %v", captured["code"], captured["code"])
	}
}

func TestRecipeService_Import_BackfillsConfigName(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/recipes":
			json.NewDecoder(r.Body).Decode(&captured)
			w.Write([]byte(`{"success":true,"id":1}`))
		case r.Method == "GET" && r.URL.Path == "/recipes/1":
			json.NewEncoder(w).Encode(Recipe{ID: 1, Name: "test", FolderID: 10})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	// Config entry missing "name" — should be backfilled from "provider".
	body := []byte(`{"name":"test","code":{},"config":[{"keyword":"application","provider":"salesforce","account_id":123}]}`)

	client := NewHTTPClient(srv.URL, "test-token")
	_, err := client.Recipes().Import(context.Background(), 10, body)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	configStr, ok := captured["config"].(string)
	if !ok {
		t.Fatalf("config not stringified; got %T", captured["config"])
	}
	var config []map[string]any
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}
	if len(config) != 1 {
		t.Fatalf("config entries = %d, want 1", len(config))
	}
	if config[0]["name"] != "salesforce" {
		t.Errorf("name = %v, want salesforce (backfilled from provider)", config[0]["name"])
	}
}

func TestRecipeService_Import_PreservesExistingConfigName(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/recipes":
			json.NewDecoder(r.Body).Decode(&captured)
			w.Write([]byte(`{"success":true,"id":1}`))
		case r.Method == "GET" && r.URL.Path == "/recipes/1":
			json.NewEncoder(w).Encode(Recipe{ID: 1, Name: "test", FolderID: 10})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	// Config entry already has "name" — should not be overwritten.
	body := []byte(`{"name":"test","code":{},"config":[{"keyword":"application","provider":"salesforce","name":"salesforce","account_id":123}]}`)

	client := NewHTTPClient(srv.URL, "test-token")
	_, err := client.Recipes().Import(context.Background(), 10, body)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	configStr, ok := captured["config"].(string)
	if !ok {
		t.Fatalf("config not stringified; got %T", captured["config"])
	}
	var config []map[string]any
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}
	if config[0]["name"] != "salesforce" {
		t.Errorf("name = %v, want salesforce (preserved)", config[0]["name"])
	}
}

func TestRecipeService_ListVersions_DataWrapper(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/recipes/42/versions" {
			t.Errorf("path = %s, want /recipes/42/versions", r.URL.Path)
		}
		if per := r.URL.Query().Get("per_page"); per != "50" {
			t.Errorf("per_page = %q, want 50", per)
		}
		w.Header().Set("Content-Type", "application/json")
		// Note the {"data": ...} wrapper, distinct from {"items": ...}
		w.Write([]byte(`{"data":[{"id":1,"version_no":2,"author_name":"Zayne","author_email":"z@x","created_at":"2026-04-13T14:55:25Z","updated_at":"2026-04-13T14:55:25Z"}]}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	vs, err := client.Recipes().ListVersions(context.Background(), 42, 0, 50)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(vs) != 1 || vs[0].VersionNo != 2 || vs[0].AuthorName != "Zayne" {
		t.Errorf("versions = %+v, want single entry parsed from data wrapper", vs)
	}
}

func TestRecipeService_GetVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/recipes/42/versions/99" {
			t.Errorf("path = %s, want /recipes/42/versions/99", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":99,"version_no":3}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	v, err := client.Recipes().GetVersion(context.Background(), 42, 99)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if v.ID != 99 || v.VersionNo != 3 {
		t.Errorf("version = %+v, want ID=99 VersionNo=3", v)
	}
}

func TestRecipeService_UpdateVersionComment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/recipes/42/versions/99" {
			t.Errorf("path = %s, want /recipes/42/versions/99", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["comment"] != "ok" {
			t.Errorf("comment = %v, want 'ok'", body["comment"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":99,"version_no":3,"comment":"ok"}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	v, err := client.Recipes().UpdateVersionComment(context.Background(), 42, 99, "ok")
	if err != nil {
		t.Fatalf("UpdateVersionComment: %v", err)
	}
	if v.Comment == nil || *v.Comment != "ok" {
		t.Errorf("comment roundtrip failed: %+v", v)
	}
}

func TestRecipeService_UpdateVersionComment_TooLong(t *testing.T) {
	client := NewHTTPClient("http://unused", "test-token")
	long := strings.Repeat("x", 256)
	_, err := client.Recipes().UpdateVersionComment(context.Background(), 42, 99, long)
	if err == nil || !strings.Contains(err.Error(), "255-character limit") {
		t.Errorf("err = %v, want 255-character limit", err)
	}
}
