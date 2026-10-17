// Coverage tests for API struct fields and CLI table output.
// See CONTRIBUTING.md for the full workflow when adding or modifying resources.
package api

import (
	"reflect"
	"testing"
)

// jsonTags extracts all json field names from a struct type,
// stripping ",omitempty" and skipping "-" tags. Recurses into
// anonymous (embedded) fields so embedded structs contribute their tags.
func jsonTags(t reflect.Type) map[string]bool {
	tags := make(map[string]bool)
	collectJSONTags(t, tags)
	return tags
}

func collectJSONTags(t reflect.Type, tags map[string]bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			collectJSONTags(f.Type, tags)
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if idx := len(tag); idx > 0 {
			for j, c := range tag {
				if c == ',' {
					tag = tag[:j]
					break
				}
			}
		}
		tags[tag] = true
	}
}

// TestStructFieldCoverage verifies that our Go structs capture all known
// fields from the Workato API. When the API adds new fields, add them to
// the expectedFields list below and then add the corresponding struct field.
//
// This is the single source of truth for "what fields should each struct have?"
// If this test fails, either:
//   - A new API field was discovered → add it to both expectedFields AND the struct
//   - A struct field was accidentally removed → restore it
func TestStructFieldCoverage(t *testing.T) {
	tests := []struct {
		name           string
		structType     reflect.Type
		expectedFields []string
	}{
		{
			name:       "Recipe",
			structType: reflect.TypeOf(Recipe{}),
			expectedFields: []string{
				"id", "name", "description", "folder_id",
				"running", "active", "version",
				"created_at", "updated_at",
				"code", "config", "trigger_application",
			},
		},
		{
			name:       "Connection",
			structType: reflect.TypeOf(Connection{}),
			expectedFields: []string{
				"id", "name", "application", "folder_id",
				"authorization_status", "authorization_error",
				"created_at", "updated_at",
			},
		},
		{
			name:       "Folder",
			structType: reflect.TypeOf(Folder{}),
			expectedFields: []string{
				"id", "name", "parent_id", "is_project", "project_id",
			},
		},
		{
			name:       "APICollection",
			structType: reflect.TypeOf(APICollection{}),
			expectedFields: []string{
				"id", "name", "version", "url",
				"api_spec_url", "project_id",
			},
		},
		{
			name:       "APIEndpoint",
			structType: reflect.TypeOf(APIEndpoint{}),
			expectedFields: []string{
				"id", "name", "api_collection_id", "active",
				"method", "path", "url", "flow_id",
				"recipe_id", "description",
			},
		},
		{
			name:       "Job",
			structType: reflect.TypeOf(Job{}),
			expectedFields: []string{
				"id", "recipe_id", "status",
				"started_at", "completed_at",
				"title", "is_error", "error", "is_poll_error",
				"calling_recipe_id", "calling_job_id",
				"root_recipe_id", "root_job_id", "master_job_id",
			},
		},
		{
			name:       "JobDetail",
			structType: reflect.TypeOf(JobDetail{}),
			expectedFields: []string{
				"id", "recipe_id", "status",
				"started_at", "completed_at",
				"title", "is_error", "error", "is_poll_error",
				"calling_recipe_id", "calling_job_id",
				"root_recipe_id", "root_job_id", "master_job_id",
				"handle", "is_repeat", "is_test", "is_test_case_job",
				"master_job_handle", "calling_job_handle", "lines",
				"error_parts", "job_correlation_id",
			},
		},
		{
			name:       "JobLine",
			structType: reflect.TypeOf(JobLine{}),
			expectedFields: []string{
				"recipe_line_number", "adapter_name", "adapter_operation",
				"line_stat", "input", "output", "error",
				"error_descriptor", "error_details",
			},
		},
		{
			name:       "LineStat",
			structType: reflect.TypeOf(LineStat{}),
			expectedFields: []string{
				"started_at", "completed_at", "total", "details",
			},
		},
		{
			name:       "ErrorDescriptor",
			structType: reflect.TypeOf(ErrorDescriptor{}),
			expectedFields: []string{
				"error_type", "error_id", "line_number", "adapter",
				"error_at", "error_type_id", "actionable", "action", "trigger",
			},
		},
		{
			name:       "ErrorDetails",
			structType: reflect.TypeOf(ErrorDetails{}),
			expectedFields: []string{
				"message", "inner_message", "http_response",
			},
		},
		{
			name:       "HTTPResponse",
			structType: reflect.TypeOf(HTTPResponse{}),
			expectedFields: []string{
				"protocol", "code", "raw_status_text", "normalized_status_text", "body", "headers",
			},
		},
		{
			name:       "ErrorParts",
			structType: reflect.TypeOf(ErrorParts{}),
			expectedFields: []string{
				"message", "error_type", "error_id", "action",
				"line_number", "adapter", "retry_count",
			},
		},
		{
			name:       "Tag",
			structType: reflect.TypeOf(Tag{}),
			expectedFields: []string{
				"handle", "title", "description", "color",
			},
		},
		{
			name:       "WorkspaceUser",
			structType: reflect.TypeOf(WorkspaceUser{}),
			expectedFields: []string{
				"id", "name", "email",
			},
		},
		{
			name:       "WorkspaceInfo",
			structType: reflect.TypeOf(WorkspaceInfo{}),
			expectedFields: []string{
				"id", "name", "email",
			},
		},
		{
			name:       "AuditLogEntry",
			structType: reflect.TypeOf(AuditLogEntry{}),
			expectedFields: []string{
				"id", "event_type", "timestamp",
				"user", "resource", "workspace", "details",
			},
		},
		{
			name:       "Connector",
			structType: reflect.TypeOf(Connector{}),
			expectedFields: []string{
				"id", "name", "title", "latest_released_version",
				"latest_released_version_note", "released_versions", "static_webhook_url",
			},
		},
		{
			name:       "ConnectorReleasedVersion",
			structType: reflect.TypeOf(ConnectorReleasedVersion{}),
			expectedFields: []string{
				"version", "version_note", "created_at", "released_at",
			},
		},
		{
			// Fields as returned by list/get/create (captured 2026-10-16). There
			// is no recipe_id or description; the recipe is provider_id.
			name:       "Skill",
			structType: reflect.TypeOf(Skill{}),
			expectedFields: []string{
				"id", "type", "name", "provider_id", "provider_type",
				"folder_id", "project_id", "folders", "running",
				"genies", "genies_count", "trigger_description", "applications",
				"created_at", "updated_at",
			},
		},
		{
			name:       "KnowledgeBase",
			structType: reflect.TypeOf(KnowledgeBase{}),
			expectedFields: []string{
				"id", "name", "description", "source_type", "folder_id", "project_id",
				"folders", "external_id", "active_recipes_count", "storage_size",
				"genies", "genies_count", "applications", "created_at", "updated_at",
			},
		},
		{
			name:       "GenieClient",
			structType: reflect.TypeOf(GenieClient{}),
			expectedFields: []string{
				"client_id", "client_name", "type", "status", "base_host",
				"genie_ids", "ip_allow_list", "activated_at",
			},
		},
		{
			name:           "Guardrail",
			structType:     reflect.TypeOf(Guardrail{}),
			expectedFields: []string{"id", "policies"},
		},
		{
			name:           "GuardrailPolicy",
			structType:     reflect.TypeOf(GuardrailPolicy{}),
			expectedFields: []string{"id", "policy_type", "configuration"},
		},
		{
			name:       "Conversation",
			structType: reflect.TypeOf(Conversation{}),
			expectedFields: []string{
				"id", "topic", "genie_handle", "genie_id", "genie_name", "genie_version",
				"app_type", "chat_type", "conversation_origin", "started_via", "started_by",
				"test_conversation", "environment_id", "environment_type", "workspace_id", "created_at",
			},
		},
		{
			name:       "ConversationDetail",
			structType: reflect.TypeOf(ConversationDetail{}),
			expectedFields: []string{
				"id", "topic", "genie_handle", "genie_id", "genie_name", "genie_version",
				"app_type", "chat_type", "conversation_origin", "started_via", "started_by",
				"test_conversation", "environment_id", "environment_type", "workspace_id", "created_at",
				"last_message_at", "totals",
			},
		},
		{
			// Common fields plus every event-type-specific field seen.
			name:       "ConversationEvent",
			structType: reflect.TypeOf(ConversationEvent{}),
			expectedFields: []string{
				"event_id", "event_type", "date_time", "conversation_id", "genie_handle",
				"genie_id", "genie_run_id", "identity_user_id", "environment_id", "workspace_id",
				"app_type", "chat_type", "request_id", "data", "genie_run_trigger",
				"first_run_in_conversation", "test_run",
			},
		},
		{
			// Fields as returned by GET /agentic/genies (captured 2026-10-16).
			name:       "Genie",
			structType: reflect.TypeOf(Genie{}),
			expectedFields: []string{
				"id", "name", "description", "state", "folder_id", "instructions",
				"ai_provider", "ai_model", "chat_interface",
				"channel_mode", "channel_scope", "channel_whitelist", "response_mode",
				"web_search_enabled", "web_search_available", "code_interpreter_enabled",
				"skills_count", "knowledge_bases_count", "user_groups_count",
				"active_recipes_count", "inactive_skills_count",
				"triggers", "logo_url", "created_at", "updated_at",
			},
		},
		{
			// GET /agentic/genies/:id adds these to the list fields.
			name:       "GenieDetail",
			structType: reflect.TypeOf(GenieDetail{}),
			expectedFields: []string{
				"id", "name", "description", "state", "folder_id", "instructions",
				"ai_provider", "ai_model", "chat_interface",
				"channel_mode", "channel_scope", "channel_whitelist", "response_mode",
				"web_search_enabled", "web_search_available", "code_interpreter_enabled",
				"skills_count", "knowledge_bases_count", "user_groups_count",
				"active_recipes_count", "inactive_skills_count",
				"triggers", "logo_url", "created_at", "updated_at",
				"project_id", "ai_model_label", "ai_provider_id", "ai_provider_type",
				"connection_id", "custom_oauth_key_id", "folders", "user_groups", "guardrails",
			},
		},
		{
			name:       "APIClient",
			structType: reflect.TypeOf(APIClient{}),
			expectedFields: []string{
				"id", "name", "auth_type", "is_legacy", "mtls_enabled",
				"active_api_keys_count", "total_api_keys_count",
				"api_collections", "api_keys",
				"created_at", "updated_at",
			},
		},
		{
			name:       "APICollectionRef",
			structType: reflect.TypeOf(APICollectionRef{}),
			expectedFields: []string{
				"id", "name",
			},
		},
		{
			name:       "APIKey",
			structType: reflect.TypeOf(APIKey{}),
			expectedFields: []string{
				"id", "name", "auth_type", "auth_token",
				"active", "active_since",
				"ip_allow_list", "ip_deny_list",
			},
		},
		{
			name:       "MCPManagedServer",
			structType: reflect.TypeOf(MCPManagedServer{}),
			expectedFields: []string{
				"id", "name", "description", "asset_type", "logo_url",
				"mcp_url", "auth_type", "authentication_method",
				"folder_id", "project_id", "folders",
				"has_vua_dependent_tools", "idp_user_group_ids",
				"api_collection", "tools_count",
				"created_at", "updated_at",
			},
		},
		{
			name:       "MCPServerFolder",
			structType: reflect.TypeOf(MCPServerFolder{}),
			expectedFields: []string{
				"id", "name",
			},
		},
		{
			name:       "MCPServerCollectionRef",
			structType: reflect.TypeOf(MCPServerCollectionRef{}),
			expectedFields: []string{
				"id", "type", "name", "created_at", "updated_at",
			},
		},
		{
			name:       "MCPServerTool",
			structType: reflect.TypeOf(MCPServerTool{}),
			expectedFields: []string{
				"id", "name", "description", "original_description",
				"trigger_application", "action_applications",
				"flow_id", "active", "enabled", "vua_required",
				"incompatibility_reasons",
			},
		},
		{
			name:       "MCPServerPolicy",
			structType: reflect.TypeOf(MCPServerPolicy{}),
			expectedFields: []string{
				"id", "mcp_server_id", "rate_limits", "quota_limits",
				"ip_allow_list", "ip_deny_list",
				"created_at", "updated_at",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := jsonTags(tt.structType)

			// Check that every expected API field is present in the struct.
			for _, field := range tt.expectedFields {
				if !tags[field] {
					t.Errorf("struct %s is missing expected API field %q — add it to the struct in types.go", tt.name, field)
				}
			}

			// Reverse check: flag struct fields not in the expected list.
			// These aren't failures — they might be intentional extras — but
			// they should be added to expectedFields to keep the list current.
			expected := make(map[string]bool)
			for _, f := range tt.expectedFields {
				expected[f] = true
			}
			for tag := range tags {
				if !expected[tag] {
					t.Errorf("struct %s has field %q not in expectedFields — add it to the test to acknowledge it", tt.name, tag)
				}
			}
		})
	}
}

// TestStructFieldCoverage_TableColumns verifies that list command table
// outputs include a minimum set of "important" fields. Fields that are
// complex objects (code, config, details) or timestamps are excluded
// since they're better consumed via --json.
//
// To use: when adding a new field to a struct, decide if it should appear
// in the table output. If yes, add it here AND to the command's FormatList call.
func TestStructFieldCoverage_TableColumns(t *testing.T) {
	// Maps resource name → minimum fields that SHOULD appear in table output.
	// Update this list when you add columns to a list command.
	requiredTableFields := map[string][]string{
		"Recipe":            {"id", "name", "description", "folder_id", "running", "version"},
		"Connection":        {"id", "name", "application", "folder_id", "authorization_status"},
		"Folder":            {"id", "name", "parent_id"},
		"APICollection":     {"id", "name", "version", "url", "project_id"},
		"APIEndpoint":       {"id", "name", "method", "path", "flow_id", "api_collection_id", "active"},
		"Tag":               {"handle", "title", "description", "color"},
		"WorkspaceUser":     {"id", "name", "email"},
		"AuditLogEntry":     {"id", "event_type", "user", "timestamp"},
		"Connector":         {"id", "name", "title", "latest_released_version"},
		"Skill":             {"id", "name", "provider_id", "folder_id", "project_id", "running"},
		"Genie":             {"id", "name", "state", "ai_model", "skills_count", "knowledge_bases_count", "folder_id"},
		"KnowledgeBase":     {"id", "name", "source_type", "active_recipes_count", "genies_count", "folder_id"},
		"GenieClient":       {"client_id", "client_name", "type", "status", "genie_ids"},
		"Conversation":      {"id", "topic", "app_type", "started_by", "created_at"},
		"ConversationEvent": {"date_time", "event_type", "genie_run_id", "data"},
		"APIClient":         {"id", "name", "auth_type", "api_collections", "active_api_keys_count"},
		"MCPManagedServer":  {"id", "name", "auth_type", "tools_count", "project_id", "folder_id"},
		"MCPServerTool":     {"id", "name", "description", "flow_id", "active"},
	}

	structTypes := map[string]reflect.Type{
		"Recipe":            reflect.TypeOf(Recipe{}),
		"Connection":        reflect.TypeOf(Connection{}),
		"Folder":            reflect.TypeOf(Folder{}),
		"APICollection":     reflect.TypeOf(APICollection{}),
		"APIEndpoint":       reflect.TypeOf(APIEndpoint{}),
		"Tag":               reflect.TypeOf(Tag{}),
		"WorkspaceUser":     reflect.TypeOf(WorkspaceUser{}),
		"AuditLogEntry":     reflect.TypeOf(AuditLogEntry{}),
		"Connector":         reflect.TypeOf(Connector{}),
		"Skill":             reflect.TypeOf(Skill{}),
		"Genie":             reflect.TypeOf(Genie{}),
		"KnowledgeBase":     reflect.TypeOf(KnowledgeBase{}),
		"GenieClient":       reflect.TypeOf(GenieClient{}),
		"Conversation":      reflect.TypeOf(Conversation{}),
		"ConversationEvent": reflect.TypeOf(ConversationEvent{}),
		"APIClient":         reflect.TypeOf(APIClient{}),
		"MCPManagedServer":  reflect.TypeOf(MCPManagedServer{}),
		"MCPServerTool":     reflect.TypeOf(MCPServerTool{}),
	}

	for name, fields := range requiredTableFields {
		t.Run(name+"_table_fields_exist_in_struct", func(t *testing.T) {
			st, ok := structTypes[name]
			if !ok {
				t.Fatalf("no struct type registered for %s", name)
			}
			tags := jsonTags(st)
			for _, field := range fields {
				if !tags[field] {
					t.Errorf("table requires field %q but struct %s doesn't have it — the table output will break", field, name)
				}
			}
		})
	}
}
