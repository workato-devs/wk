package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type recipeService struct {
	client *HTTPClient
}

func (s *recipeService) List(ctx context.Context, opts *RecipeListOptions) ([]Recipe, error) {
	params := url.Values{}
	if opts != nil {
		if opts.FolderID != nil {
			params.Set("folder_id", strconv.Itoa(*opts.FolderID))
		}
		switch opts.Status {
		case "running":
			params.Set("active", "true")
		case "stopped":
			params.Set("active", "false")
		}
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := "/recipes"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result ListResult[Recipe]
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *recipeService) Get(ctx context.Context, id int) (*Recipe, error) {
	var recipe Recipe
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/recipes/%d", id), nil, &recipe); err != nil {
		return nil, err
	}
	return &recipe, nil
}

func (s *recipeService) Start(ctx context.Context, id int) error {
	body, err := s.client.doRaw(ctx, "PUT", fmt.Sprintf("/recipes/%d/start", id), nil)
	if err != nil {
		return err
	}
	return parseActivationError(id, body)
}

func (s *recipeService) Stop(ctx context.Context, id int) error {
	return s.client.do(ctx, "PUT", fmt.Sprintf("/recipes/%d/stop", id), nil, nil)
}

func (s *recipeService) Export(ctx context.Context, id int) ([]byte, error) {
	return s.client.doRaw(ctx, "GET", fmt.Sprintf("/recipes/%d", id), nil)
}

func (s *recipeService) Import(ctx context.Context, folderID int, data []byte) (*Recipe, error) {
	body, err := s.decodeAndResolve(ctx, data)
	if err != nil {
		return nil, err
	}
	body["folder_id"] = strconv.Itoa(folderID)

	var result struct {
		Success bool `json:"success"`
		ID      int  `json:"id"`
	}
	if err := s.client.do(ctx, "POST", "/recipes", body, &result); err != nil {
		return nil, err
	}
	return s.Get(ctx, result.ID)
}

// Update replaces an existing recipe's code/config via PUT /recipes/{id}.
// Shares the same stringification rules as Import — the Workato API expects
// "code" and "config" as JSON-encoded strings even though exports return
// them as objects.
func (s *recipeService) Update(ctx context.Context, id int, data []byte) error {
	body, err := s.decodeAndResolve(ctx, data)
	if err != nil {
		return err
	}
	// folder_id is meaningful only on create; drop it so callers can reuse
	// their export JSON without accidentally moving the recipe.
	delete(body, "folder_id")
	resp, err := s.client.doRaw(ctx, "PUT", fmt.Sprintf("/recipes/%d", id), body)
	if err != nil {
		return err
	}
	return parseMutationRefusal(fmt.Sprintf("updating recipe %d", id), resp)
}

// Delete removes a recipe via DELETE /recipes/{id}. The endpoint returns
// HTTP 200 for both outcomes; a refusal (e.g. the recipe is running)
// carries success:false in the body.
func (s *recipeService) Delete(ctx context.Context, id int) error {
	resp, err := s.client.doRaw(ctx, "DELETE", fmt.Sprintf("/recipes/%d", id), nil)
	if err != nil {
		return err
	}
	return parseMutationRefusal(fmt.Sprintf("deleting recipe %d", id), resp)
}

// Move changes a recipe's folder via PUT /recipes/{id} with an explicit
// folder_id. Update deliberately strips folder_id so reused export JSON
// cannot move a recipe by accident; Move is the explicit opt-in. It re-sends
// the recipe's current code/config (fetched via Export) alongside the new
// folder_id, so the recipe ID, job history, and external references are all
// preserved — unlike copy+delete, which mints a new ID.
func (s *recipeService) Move(ctx context.Context, id, folderID int) error {
	data, err := s.Export(ctx, id)
	if err != nil {
		return fmt.Errorf("fetching recipe %d: %w", id, err)
	}
	body, err := s.decodeAndResolve(ctx, data)
	if err != nil {
		return err
	}
	body["folder_id"] = strconv.Itoa(folderID)
	resp, err := s.client.doRaw(ctx, "PUT", fmt.Sprintf("/recipes/%d", id), body)
	if err != nil {
		return err
	}
	return parseMutationRefusal(fmt.Sprintf("moving recipe %d", id), resp)
}

// CanonicalizeRecipeExport converts the raw GET /recipes/{id} response into
// the project recipe file format produced by the package-export path
// (wk pull) and required by wk lint. The single-recipe endpoint differs from
// the package format in ways that otherwise make its output fail lint:
//
//   - "code" is returned as an escaped JSON string; lint requires an object.
//   - the recipe version is exposed as "version_no"; the project format key
//     is "version".
//   - "private" and "concurrency" are not returned by this endpoint at all,
//     so they fall back to the platform defaults (false / 1). Faithful values
//     for these two require the package export (wk pull).
//
// Runtime-only fields from the GET response (job counts, webhook_url, etc.)
// are intentionally dropped: a project recipe file is a definition, not a
// status snapshot, matching what wk pull writes.
func CanonicalizeRecipeExport(raw []byte) ([]byte, error) {
	var src map[string]any
	if err := json.Unmarshal(raw, &src); err != nil {
		return nil, fmt.Errorf("parsing recipe export: %w", err)
	}

	out := map[string]any{
		"name":        src["name"],
		"description": src["description"],
		"private":     false,
		"concurrency": 1,
	}

	if v, ok := src["config"]; ok {
		out["config"] = v
	} else {
		out["config"] = []any{}
	}

	// code: parse the escaped JSON string into an object. If it is already an
	// object (defensive — a future API change), keep it as-is.
	switch c := src["code"].(type) {
	case string:
		var decoded any
		if err := json.Unmarshal([]byte(c), &decoded); err != nil {
			return nil, fmt.Errorf("parsing recipe \"code\" string: %w", err)
		}
		out["code"] = decoded
	case nil:
		// No code field — don't fabricate one; lint will surface it.
	default:
		out["code"] = c
	}

	// version: prefer an explicit "version", else map the endpoint's
	// "version_no".
	if v, ok := src["version"]; ok {
		out["version"] = v
	} else if v, ok := src["version_no"]; ok {
		out["version"] = v
	} else {
		out["version"] = 0
	}

	// Honor real values if a future API revision starts returning them.
	if v, ok := src["private"]; ok {
		out["private"] = v
	}
	if v, ok := src["concurrency"]; ok {
		out["concurrency"] = v
	}

	formatted, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(formatted, '\n'), nil
}

// decodeAndResolve unmarshals recipe JSON, resolves any connection reference
// objects in config to integer IDs, backfills missing "name" fields from
// "provider", then stringifies code/config for the API.
func (s *recipeService) decodeAndResolve(ctx context.Context, data []byte) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("invalid recipe JSON: %w", err)
	}

	if err := s.resolveConfigConnections(ctx, body); err != nil {
		return nil, err
	}
	backfillConfigNames(body)

	for _, key := range []string{"code", "config"} {
		v, ok := body[key]
		if !ok {
			continue
		}
		if _, isString := v.(string); isString {
			continue
		}
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("encoding %s: %w", key, err)
		}
		body[key] = string(encoded)
	}
	return body, nil
}

// resolveConfigConnections walks config entries and replaces any account_id
// that is a connection reference object (from ZIP/package export format) with
// the integer connection ID from the workspace. The per-recipe API endpoints
// expect integer account_id values; the ZIP import endpoint resolves these
// server-side, but PUT/POST /recipes do not.
func (s *recipeService) resolveConfigConnections(ctx context.Context, body map[string]any) error {
	configVal, ok := body["config"]
	if !ok {
		return nil
	}

	configSlice, ok := configVal.([]any)
	if !ok {
		return nil
	}

	// Scan for any object-typed account_id before hitting the API.
	needsResolution := false
	for _, entry := range configSlice {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if _, isObj := entryMap["account_id"].(map[string]any); isObj {
			needsResolution = true
			break
		}
	}
	if !needsResolution {
		return nil
	}

	connSvc := &connectionService{client: s.client}
	conns, err := connSvc.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("listing connections to resolve config references: %w", err)
	}

	byName := make(map[string]int, len(conns))
	for _, c := range conns {
		byName[c.Name] = c.ID
	}

	for _, entry := range configSlice {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		ref, ok := entryMap["account_id"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := ref["name"].(string)
		if name == "" {
			return fmt.Errorf("connection reference missing \"name\" in config entry (provider=%v)", entryMap["provider"])
		}
		id, found := byName[name]
		if !found {
			return fmt.Errorf("connection %q not found in workspace; cannot resolve account_id for provider %v", name, entryMap["provider"])
		}
		entryMap["account_id"] = id
	}

	return nil
}

// backfillConfigNames ensures every config entry has a "name" field. The
// Workato platform requires it to wire adapters on activation; exports always
// set name == provider, but hand-crafted or ZIP-extracted JSON may omit it,
// causing a silent activation failure.
func backfillConfigNames(body map[string]any) {
	configVal, ok := body["config"]
	if !ok {
		return
	}
	configSlice, ok := configVal.([]any)
	if !ok {
		return
	}
	for _, entry := range configSlice {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if _, hasName := entryMap["name"]; hasName {
			continue
		}
		if provider, ok := entryMap["provider"].(string); ok && provider != "" {
			entryMap["name"] = provider
		}
	}
}

func (s *recipeService) ListJobs(ctx context.Context, recipeID int, opts *JobListOptions) ([]Job, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Status != "" && opts.Status != "all" {
			params.Set("status", opts.Status)
		}
		if opts.Limit > 0 {
			params.Set("per_page", strconv.Itoa(opts.Limit))
		}
	}
	path := fmt.Sprintf("/recipes/%d/jobs", recipeID)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result ListResult[Job]
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *recipeService) GetJob(ctx context.Context, recipeID int, jobID string) (*JobDetail, error) {
	var detail JobDetail
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/recipes/%d/jobs/%s", recipeID, url.PathEscape(jobID)), nil, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (s *recipeService) Copy(ctx context.Context, recipeID, folderID int) (*Recipe, error) {
	// The API rejects a numeric folder_id ("Must be a String"); send it as a
	// string, matching Import/Move.
	body := map[string]any{"folder_id": strconv.Itoa(folderID)}
	// The copy endpoint returns {"success": true, "new_flow_id": N}, not a
	// recipe object — decode the new id and fetch the full recipe, mirroring
	// Import.
	var result struct {
		Success   bool `json:"success"`
		NewFlowID int  `json:"new_flow_id"`
	}
	if err := s.client.do(ctx, "POST", fmt.Sprintf("/recipes/%d/copy", recipeID), body, &result); err != nil {
		return nil, err
	}
	return s.Get(ctx, result.NewFlowID)
}

// ListVersions returns the version history for a recipe. Pagination matches
// the Workato contract: default page size 100, max 100. page/perPage values
// <= 0 omit the query params so the server applies its defaults.
//
// Note the response wrapper is `{"data": [...]}`, not `{"items": [...]}`
// like most list endpoints — the generic ListResult[T] would not decode
// it, so the wrapper is inline.
func (s *recipeService) ListVersions(ctx context.Context, recipeID, page, perPage int) ([]RecipeVersion, error) {
	params := url.Values{}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		params.Set("per_page", strconv.Itoa(perPage))
	}
	path := fmt.Sprintf("/recipes/%d/versions", recipeID)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result struct {
		Data []RecipeVersion `json:"data"`
	}
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// GetVersion returns a single version's metadata.
func (s *recipeService) GetVersion(ctx context.Context, recipeID, versionID int) (*RecipeVersion, error) {
	var v RecipeVersion
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/recipes/%d/versions/%d", recipeID, versionID), nil, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// UpdateVersionComment sets a comment on a specific version via PATCH.
// The API caps the comment at 255 characters; the check is enforced here
// so the caller sees a clear local error instead of an API 4xx.
func (s *recipeService) UpdateVersionComment(ctx context.Context, recipeID, versionID int, comment string) (*RecipeVersion, error) {
	if len(comment) > 255 {
		return nil, fmt.Errorf("comment exceeds 255-character limit (got %d)", len(comment))
	}
	body := map[string]any{"comment": comment}
	var v RecipeVersion
	if err := s.client.do(ctx, "PATCH", fmt.Sprintf("/recipes/%d/versions/%d", recipeID, versionID), body, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *recipeService) Connect(ctx context.Context, recipeID int, adapterName string, connectionID int) error {
	body := map[string]any{
		"adapter_name":  adapterName,
		"connection_id": connectionID,
	}
	return s.client.do(ctx, "PUT", fmt.Sprintf("/recipes/%d/connect", recipeID), body, nil)
}

func (s *recipeService) RepeatJobs(ctx context.Context, recipeID int, jobIDs []string) (*RepeatJobsResult, error) {
	body := map[string]any{"job_ids": jobIDs}
	var result RepeatJobsResult
	if err := s.client.do(ctx, "POST", fmt.Sprintf("/recipes/%d/repeat_jobs", recipeID), body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ActivationError reports why the platform refused to activate a recipe.
// PUT /recipes/{id}/start returns HTTP 200 with success:false and a
// code_errors payload when activation is blocked; see the fixtures in
// recipes_test.go for recorded shapes.
type ActivationError struct {
	RecipeID     int
	CodeErrors   []StepCodeErrors
	ConfigErrors []StepCodeErrors
}

// StepCodeErrors groups activation errors for one recipe step (by step number).
type StepCodeErrors struct {
	Step    int
	Details []FieldCodeError
}

// FieldCodeError is one field-level activation error within a step.
type FieldCodeError struct {
	Label   string
	Value   any
	Message string
	Path    string
}

func (e *ActivationError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "recipe %d cannot activate: the platform reported step errors", e.RecipeID)
	for _, step := range append(append([]StepCodeErrors{}, e.CodeErrors...), e.ConfigErrors...) {
		for _, d := range step.Details {
			fmt.Fprintf(&b, "\n  step %d: %s %s", step.Step, d.Label, d.Message)
			if d.Path != "" {
				fmt.Fprintf(&b, " (%s)", d.Path)
			}
		}
	}
	return b.String()
}

// startResponse is the (undocumented) body of PUT /recipes/{id}/start. The
// endpoint returns HTTP 200 for both outcomes; success:false carries the
// activation errors that the recipe editor shows as inline step annotations.
type startResponse struct {
	Success      bool             `json:"success"`
	CodeErrors   []StepCodeErrors `json:"code_errors"`
	ConfigErrors []StepCodeErrors `json:"config_errors"`
}

// UnmarshalJSON decodes the positional pair [step_number, [field errors]].
func (s *StepCodeErrors) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) < 2 {
		return fmt.Errorf("code_errors entry: want [step, details], got %d elements", len(raw))
	}
	if err := json.Unmarshal(raw[0], &s.Step); err != nil {
		return fmt.Errorf("code_errors step number: %w", err)
	}
	return json.Unmarshal(raw[1], &s.Details)
}

// UnmarshalJSON decodes the positional tuple [label, current_value, message]
// with an optional fourth path element. Observed live shapes: schema errors
// carry four elements ([label, value, message, path]); invalid-name errors
// carry three; config_errors reuse the layout with a non-string fourth
// element, so the tail is decoded best-effort.
func (f *FieldCodeError) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) < 3 {
		return fmt.Errorf("code_errors detail: want at least 3 elements, got %d", len(raw))
	}
	if err := json.Unmarshal(raw[0], &f.Label); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &f.Value); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[2], &f.Message); err != nil {
		return err
	}
	if len(raw) > 3 {
		// Path is absent on some shapes and non-string on others; never let
		// the tail kill the details we already have.
		_ = json.Unmarshal(raw[3], &f.Path)
	}
	return nil
}

// parseActivationError inspects a start response body and returns an
// *ActivationError when the platform refused activation. The body shape is
// undocumented, so parsing is strictly best-effort: anything that doesn't
// decode as success:false keeps the previous behavior (nil — callers fall
// through to polling and the existing timeout diagnostics).
func parseActivationError(recipeID int, body []byte) error {
	// Two-pass parse: "success" alone decides blocked-or-not; the error
	// details are decoded separately so a drifted code_errors shape can
	// never suppress a definite refusal.
	var probe struct {
		Success *bool `json:"success"`
	}
	if err := json.Unmarshal(body, &probe); err != nil || probe.Success == nil || *probe.Success {
		return nil
	}
	actErr := &ActivationError{RecipeID: recipeID}
	var resp startResponse
	if err := json.Unmarshal(body, &resp); err == nil {
		actErr.CodeErrors = resp.CodeErrors
		actErr.ConfigErrors = resp.ConfigErrors
	}
	return actErr
}

// MutationRefusedError reports a 2xx mutation response whose body carries
// success:false — the platform acknowledged the request but refused to apply
// it (e.g. deleting or updating a recipe that is currently running). Several
// recipe lifecycle endpoints use this shape instead of a 4xx status.
type MutationRefusedError struct {
	Op      string
	Reasons []string
}

func (e *MutationRefusedError) Error() string {
	if len(e.Reasons) == 0 {
		return e.Op + ": refused by the platform (success:false)"
	}
	return e.Op + ": " + strings.Join(e.Reasons, "; ")
}

// parseMutationRefusal inspects a 2xx mutation response body for the
// {"success":false,"errors":{field:[messages]}} refusal shape. Best-effort,
// mirroring parseActivationError: anything that doesn't decode as
// success:false keeps the previous ignore-the-body behavior.
func parseMutationRefusal(op string, body []byte) error {
	var probe struct {
		Success *bool               `json:"success"`
		Errors  map[string][]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &probe); err != nil || probe.Success == nil || *probe.Success {
		return nil
	}
	e := &MutationRefusedError{Op: op}
	for _, msgs := range probe.Errors {
		e.Reasons = append(e.Reasons, msgs...)
	}
	sort.Strings(e.Reasons)
	return e
}
