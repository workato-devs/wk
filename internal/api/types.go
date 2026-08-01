package api

import (
	"encoding/json"
	"time"
)

// Recipe represents a Workato recipe.
type Recipe struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	FolderID    int       `json:"folder_id"`
	Running     bool      `json:"running"`
	Active      bool      `json:"active"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Code        any       `json:"code,omitempty"`
	Config      any       `json:"config,omitempty"`
	// TriggerApplication identifies the recipe's trigger connector (e.g.
	// "workato_api_platform"). The MCP tools API keys recipe tools on it.
	TriggerApplication string `json:"trigger_application,omitempty"`
}

// RecipeVersion represents a single entry in a recipe's version history
// (GET /recipes/:id/versions). Comment is a pointer because the API may
// return null for versions that were never commented; *string preserves
// the distinction between "no comment" and "empty comment".
type RecipeVersion struct {
	ID          int       `json:"id"`
	VersionNo   int       `json:"version_no"`
	Comment     *string   `json:"comment,omitempty"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Connection represents a Workato connection.
type Connection struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	Application         string    `json:"application"`
	FolderID            int       `json:"folder_id"`
	AuthorizationStatus *string   `json:"authorization_status"`
	AuthorizationError  *string   `json:"authorization_error"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Folder represents a Workato folder. IsProject distinguishes top-level
// projects from plain folders — the Workato workspace treats them as
// the same resource shape on list, but delete routes differently:
// projects require DELETE /projects/{project_id}; plain folders use
// DELETE /folders/{id}.
//
// ProjectID is populated by the list response when IsProject is true
// and is the identifier that DELETE /projects/... requires — distinct
// from ID (the folder id) even when the folder IS a project.
type Folder struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ParentID  *int   `json:"parent_id,omitempty"`
	IsProject bool   `json:"is_project,omitempty"`
	ProjectID int    `json:"project_id,omitempty"`
}

// Package represents an RLCM export/import package.
type Package struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	ErrorParts []any     `json:"error_parts,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ExportManifest represents a Workato RLCM export manifest.
// Creating a manifest is required before triggering a package export.
type ExportManifest struct {
	ID       int    `json:"id"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status,omitempty"`
	FolderID int    `json:"folder_id,omitempty"`
}

// PackageContent describes a single asset within an RLCM package.
type PackageContent struct {
	AbsolutePath string `json:"absolute_path"`
	ZipName      string `json:"zip_name"`
	Folder       string `json:"folder"`
	Type         string `json:"type"` // "recipe", "connection", etc.
}

// Job represents a recipe job execution. Job IDs are strings
// (e.g. "j-AJMfQh8c-hsCXcs"); recipe IDs are integers.
type Job struct {
	ID              string     `json:"id"`
	RecipeID        int        `json:"recipe_id"`
	Status          string     `json:"status"` // "succeeded", "failed", "pending"
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Title           string     `json:"title,omitempty"`
	IsError         bool       `json:"is_error"`
	Error           *string    `json:"error,omitempty"`
	IsPollError     bool       `json:"is_poll_error"`
	CallingRecipeID *int       `json:"calling_recipe_id,omitempty"`
	CallingJobID    *string    `json:"calling_job_id,omitempty"`
	RootRecipeID    *int       `json:"root_recipe_id,omitempty"`
	RootJobID       *string    `json:"root_job_id,omitempty"`
	MasterJobID     *string    `json:"master_job_id,omitempty"`
}

// JobDetail is the single-job response from GET /recipes/{id}/jobs/{job_id}.
type JobDetail struct {
	Job
	Handle           string      `json:"handle,omitempty"`
	IsRepeat         bool        `json:"is_repeat"`
	IsTest           bool        `json:"is_test"`
	IsTestCaseJob    bool        `json:"is_test_case_job"`
	MasterJobHandle  string      `json:"master_job_handle,omitempty"`
	CallingJobHandle string      `json:"calling_job_handle,omitempty"`
	Lines            []JobLine   `json:"lines,omitempty"`
	ErrorParts       *ErrorParts `json:"error_parts,omitempty"`
	JobCorrelationID string      `json:"job_correlation_id,omitempty"`
}

// JobLine represents a single step in a job execution trace. Beyond the
// step identity and timing, the API returns the step's input/output data
// and, on failures, full error diagnostics (error_details.http_response).
// Input/Output are held as raw JSON because their shape is per-adapter.
type JobLine struct {
	RecipeLineNumber int              `json:"recipe_line_number"`
	AdapterName      string           `json:"adapter_name"`
	AdapterOperation string           `json:"adapter_operation"`
	LineStat         *LineStat        `json:"line_stat,omitempty"`
	Input            json.RawMessage  `json:"input,omitempty"`
	Output           json.RawMessage  `json:"output,omitempty"`
	Error            *string          `json:"error,omitempty"`
	ErrorDescriptor  *ErrorDescriptor `json:"error_descriptor,omitempty"`
	ErrorDetails     *ErrorDetails    `json:"error_details,omitempty"`
}

// LineStat holds timing data for a job step. Total is the step's total
// duration in seconds (a fractional value, e.g. 0.0079); Details breaks
// that down into sub-phases.
type LineStat struct {
	StartedAt   *time.Time       `json:"started_at,omitempty"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	Total       *float64         `json:"total,omitempty"`
	Details     []LineStatDetail `json:"details,omitempty"`
}

// LineStatDetail is one sub-phase of a step's timing breakdown. The API
// reports each sub-phase as a set of duration metrics in seconds (Count is
// the sample count); there is no scalar "value" field.
type LineStatDetail struct {
	Name    string   `json:"name,omitempty"`
	Count   *int     `json:"count,omitempty"`
	Average *float64 `json:"average,omitempty"`
	Total   *float64 `json:"total,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
}

// ErrorDescriptor carries a structured error classification for a failed
// job step, when the API provides one.
type ErrorDescriptor struct {
	ErrorType   string     `json:"error_type,omitempty"`
	ErrorID     string     `json:"error_id,omitempty"`
	LineNumber  *int       `json:"line_number,omitempty"`
	Adapter     string     `json:"adapter,omitempty"`
	ErrorAt     *time.Time `json:"error_at,omitempty"`
	ErrorTypeID *string    `json:"error_type_id,omitempty"`
	Actionable  bool       `json:"actionable,omitempty"`
	Action      *string    `json:"action,omitempty"`
	Trigger     *string    `json:"trigger,omitempty"`
}

// ErrorDetails holds the diagnostic payload for a failed step, most
// importantly the downstream HTTP response that caused the failure.
type ErrorDetails struct {
	Message      string        `json:"message,omitempty"`
	InnerMessage *string       `json:"inner_message,omitempty"`
	HTTPResponse *HTTPResponse `json:"http_response,omitempty"`
}

// HTTPResponse is the downstream HTTP response captured on a step failure
// (e.g. a 401 from a called API). Headers are returned as raw JSON since
// the key set is arbitrary.
type HTTPResponse struct {
	Protocol             string          `json:"protocol,omitempty"`
	Code                 int             `json:"code,omitempty"`
	RawStatusText        string          `json:"raw_status_text,omitempty"`
	NormalizedStatusText string          `json:"normalized_status_text,omitempty"`
	Body                 string          `json:"body,omitempty"`
	Headers              json.RawMessage `json:"headers,omitempty"`
}

// ErrorParts is the job-level structured error breakdown returned
// alongside the flat error string.
type ErrorParts struct {
	Message    string `json:"message,omitempty"`
	ErrorType  string `json:"error_type,omitempty"`
	ErrorID    string `json:"error_id,omitempty"`
	Action     string `json:"action,omitempty"`
	LineNumber *int   `json:"line_number,omitempty"`
	Adapter    string `json:"adapter,omitempty"`
	RetryCount int    `json:"retry_count,omitempty"`
}

// ListResult is a generic wrapper for paginated API responses
// that return {"items":[...]}, e.g. recipes and jobs.
type ListResult[T any] struct {
	Items []T `json:"items"`
}

// Tag represents a Workato tag.
type Tag struct {
	Handle      string `json:"handle"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

// TagListOptions configures tag list filtering.
type TagListOptions struct {
	Search  string
	Page    int
	PerPage int
}

// TagUpdateOptions configures tag updates.
type TagUpdateOptions struct {
	Title       *string
	Description *string
	Color       *string
}

// APICollection represents a Workato API collection. The API does not
// support description on collections (silently ignored), and project_id
// is nullable (omitted when the collection has no project association).
type APICollection struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Version    string `json:"version,omitempty"`
	URL        string `json:"url,omitempty"`
	APISpecURL string `json:"api_spec_url,omitempty"`
	ProjectID  *int   `json:"project_id,omitempty"`
}

// APIEndpoint represents a Workato API endpoint. The API consistently uses
// "flow_id" for the recipe association (both list and create). RecipeID is
// backfilled from FlowID for display convenience.
type APIEndpoint struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	APICollectionID int     `json:"api_collection_id"`
	Active          bool    `json:"active"`
	Method          string  `json:"method,omitempty"`
	Path            string  `json:"path,omitempty"`
	URL             string  `json:"url,omitempty"`
	FlowID          int     `json:"flow_id,omitempty"`
	RecipeID        int     `json:"recipe_id,omitempty"`
	Description     *string `json:"description,omitempty"`
}

// Skill represents a Workato agentic skill. The API returns string IDs
// (e.g. "skl-Aa6zhmTh-4ac8TH-AB") and uses "provider_id" for the recipe
// association; RecipeID is backfilled from ProviderID for display convenience.
type Skill struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Description        string `json:"description,omitempty"`
	RecipeID           int    `json:"recipe_id,omitempty"`
	ProviderID         int    `json:"provider_id"`
	ProviderType       string `json:"provider_type,omitempty"`
	FolderID           int    `json:"folder_id"`
	ProjectID          int    `json:"project_id"`
	Running            bool   `json:"running"`
	GeniesCount        int    `json:"genies_count"`
	TriggerDescription string `json:"trigger_description,omitempty"`
	Applications       []any  `json:"applications,omitempty"`
}

// APIClient represents a Workato API Platform client (v2 API).
// The API wraps responses in {"data":...}; unwrapping happens in the service layer.
type APIClient struct {
	ID                 int                `json:"id"`
	Name               string             `json:"name"`
	AuthType           string             `json:"auth_type,omitempty"`
	IsLegacy           bool               `json:"is_legacy"`
	MTLSEnabled        bool               `json:"mtls_enabled"`
	ActiveAPIKeysCount int                `json:"active_api_keys_count"`
	TotalAPIKeysCount  int                `json:"total_api_keys_count"`
	APICollections     []APICollectionRef `json:"api_collections,omitempty"`
	APIKeys            []APIKey           `json:"api_keys,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

// APICollectionRef is a lightweight reference to a collection embedded in an API client response.
type APICollectionRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// APIKey represents an API key belonging to an API client (v2 API).
// AuthToken is only populated on create — subsequent reads omit it.
type APIKey struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	AuthType    string   `json:"auth_type,omitempty"`
	AuthToken   string   `json:"auth_token,omitempty"`
	Active      bool     `json:"active"`
	ActiveSince *string  `json:"active_since,omitempty"`
	IPAllowList []string `json:"ip_allow_list,omitempty"`
	IPDenyList  []string `json:"ip_deny_list,omitempty"`
}

// PaginationOptions provides generic pagination parameters.
type PaginationOptions struct {
	Page    int
	PerPage int
}

// MCPServerInfo represents the result of an MCP initialize handshake.
type MCPServerInfo struct {
	Name            string         `json:"name"`
	Version         string         `json:"version"`
	ProtocolVersion string         `json:"protocol_version"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
}

// MCPTool represents a tool exposed by an MCP server.
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

// MCPManagedServer is the full detail shape from GET /api/mcp/mcp_servers/:handle.
// The API wraps responses in {"data":...}; unwrapping happens in the service layer.
// IDs are strings (e.g. "mcps-AYcNrsC8-Dd8-AB").
type MCPManagedServer struct {
	ID                   string                  `json:"id"`
	Name                 string                  `json:"name"`
	Description          string                  `json:"description,omitempty"`
	AssetType            string                  `json:"asset_type,omitempty"`
	LogoURL              *string                 `json:"logo_url,omitempty"`
	MCPURL               string                  `json:"mcp_url,omitempty"`
	AuthType             string                  `json:"auth_type,omitempty"`
	AuthenticationMethod string                  `json:"authentication_method,omitempty"`
	FolderID             int                     `json:"folder_id"`
	ProjectID            int                     `json:"project_id"`
	Folders              []MCPServerFolder       `json:"folders,omitempty"`
	HasVUADependentTools bool                    `json:"has_vua_dependent_tools"`
	IDPUserGroupIDs      []string                `json:"idp_user_group_ids,omitempty"`
	APICollection        *MCPServerCollectionRef `json:"api_collection,omitempty"`
	ToolsCount           int                     `json:"tools_count"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

// MCPServerFolder is a lightweight folder reference embedded in an MCP server response.
type MCPServerFolder struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// MCPServerCollectionRef is the API collection linked to an MCP server.
type MCPServerCollectionRef struct {
	ID        int       `json:"id"`
	Type      string    `json:"type,omitempty"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MCPServerTool represents a tool assigned to an MCP managed server.
type MCPServerTool struct {
	ID                     int      `json:"id"`
	Name                   string   `json:"name"`
	Description            *string  `json:"description,omitempty"`
	OriginalDescription    *string  `json:"original_description,omitempty"`
	TriggerApplication     *string  `json:"trigger_application,omitempty"`
	ActionApplications     []string `json:"action_applications,omitempty"`
	FlowID                 int      `json:"flow_id"`
	Active                 bool     `json:"active"`
	Enabled                bool     `json:"enabled"`
	VUARequired            bool     `json:"vua_required"`
	IncompatibilityReasons []string `json:"incompatibility_reasons,omitempty"`
}

// MCPServerPolicy represents rate/quota limits and IP restrictions for an MCP server.
type MCPServerPolicy struct {
	ID          *int    `json:"id"`
	MCPServerID *string `json:"mcp_server_id"`
	// RateLimits/QuotaLimits are {"limit": <int>, "interval": <string>}
	// objects; the values are mixed types, so decode into map[string]any.
	RateLimits  map[string]any `json:"rate_limits,omitempty"`
	QuotaLimits map[string]any `json:"quota_limits,omitempty"`
	IPAllowList []string       `json:"ip_allow_list,omitempty"`
	IPDenyList  []string       `json:"ip_deny_list,omitempty"`
	CreatedAt   *time.Time     `json:"created_at,omitempty"`
	UpdatedAt   *time.Time     `json:"updated_at,omitempty"`
}

// MCPUserGroup is an identity-provider user group from GET /api/mcp/user_groups.
// IDs are strings (e.g. "group-abc123") and are what assign/remove_user_groups
// expect in idp_user_group_ids.
type MCPUserGroup struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	UsersCount int       `json:"users_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MCPServerListOptions configures MCP server list filtering.
type MCPServerListOptions struct {
	ProjectID            *int
	FolderID             *int
	AuthenticationMethod string
	Page                 int
	PerPage              int
}

// WorkspaceInfo is the shape returned by GET /users/me. Despite the endpoint
// path, the response describes the workspace the token authenticates against:
// id and name are the workspace's. Email is the authenticated account's email.
type WorkspaceInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// WorkspaceUser represents a Workato workspace member (from GET /members).
type WorkspaceUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// AuditLogEntry represents a Workato audit log entry.
type AuditLogEntry struct {
	ID        int    `json:"id"`
	EventType string `json:"event_type"`
	Timestamp string `json:"timestamp"`
	User      struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
	Details any `json:"details,omitempty"`
}

// AuditLogOptions configures audit log filtering.
type AuditLogOptions struct {
	Since  string
	Until  string
	Action string
}

// JobListOptions configures job list filtering.
type JobListOptions struct {
	Status string
	Limit  int
}

// RepeatJobsResult is the response from POST /recipes/:id/repeat_jobs.
type RepeatJobsResult struct {
	Results []RepeatJobEntry `json:"results"`
}

// RepeatJobEntry describes the outcome of a single job repeat request.
type RepeatJobEntry struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"` // "enqueued" or "failed"
	Error  string `json:"error,omitempty"`
}

// Connector represents a Workato connector (integration).
type Connector struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}
