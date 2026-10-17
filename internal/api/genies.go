package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// GenieListOptions filters and pages GET /agentic/genies.
type GenieListOptions struct {
	FolderID  string
	ProjectID string
	State     string
	Page      int
	PerPage   int // max 50
}

type genieService struct {
	client *HTTPClient
}

type genieListResponse struct {
	Data []Genie `json:"data"`
	OffsetPage
}

// List returns one page of genies and where that page stands.
func (s *genieService) List(ctx context.Context, opts *GenieListOptions) ([]Genie, OffsetPage, error) {
	params := url.Values{}
	if opts != nil {
		if opts.FolderID != "" {
			params.Set("folder_id", opts.FolderID)
		}
		if opts.ProjectID != "" {
			params.Set("project_id", opts.ProjectID)
		}
		if opts.State != "" {
			params.Set("state", opts.State)
		}
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := "/agentic/genies"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result genieListResponse
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, OffsetPage{}, err
	}
	return result.Data, result.OffsetPage, nil
}

// Get returns one genie with its detail-only fields.
func (s *genieService) Get(ctx context.Context, id string) (*GenieDetail, error) {
	var result struct {
		Data GenieDetail `json:"data"`
	}
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/agentic/genies/%s", url.PathEscape(id)), nil, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GenieInput is the body of a genie create or update. Nil fields aren't
// sent, so an update changes only what's set.
type GenieInput struct {
	Name             *string   `json:"name,omitempty"`
	Description      *string   `json:"description,omitempty"`
	FolderID         *string   `json:"folder_id,omitempty"`
	Instructions     *string   `json:"instructions,omitempty"`
	AIProvider       *string   `json:"ai_provider,omitempty"`
	SharedAccountID  *int      `json:"shared_account_id,omitempty"`
	CustomOAuthKeyID *int      `json:"custom_oauth_key_id,omitempty"`
	Matrix           *bool     `json:"matrix,omitempty"`
	ChannelMode      *string   `json:"channel_mode,omitempty"`
	ChannelScope     *string   `json:"channel_scope,omitempty"`
	ResponseMode     *string   `json:"response_mode,omitempty"`
	ChannelWhitelist *[]string `json:"channel_whitelist,omitempty"`
}

// GenieAssignment is what a genie can be assigned: skills, knowledge bases,
// or user groups. Its value is the API path segment.
type GenieAssignment string

const (
	AssignSkills         GenieAssignment = "skills"
	AssignKnowledgeBases GenieAssignment = "knowledge_bases"
	AssignUserGroups     GenieAssignment = "user_groups"
)

func (a GenieAssignment) idsField() string {
	switch a {
	case AssignSkills:
		return "skill_ids"
	case AssignKnowledgeBases:
		return "knowledge_base_ids"
	default:
		return "user_group_ids"
	}
}

// Create creates a genie and returns its ID; the API returns nothing else.
func (s *genieService) Create(ctx context.Context, in GenieInput) (string, error) {
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := s.client.do(ctx, "POST", "/agentic/genies", in, &result); err != nil {
		return "", err
	}
	return result.Data.ID, nil
}

// Update changes the set fields of a genie. The API returns no body.
func (s *genieService) Update(ctx context.Context, id string, in GenieInput) error {
	return s.client.do(ctx, "PUT", "/agentic/genies/"+url.PathEscape(id), in, nil)
}

func (s *genieService) Delete(ctx context.Context, id string) error {
	return s.client.do(ctx, "DELETE", "/agentic/genies/"+url.PathEscape(id), nil, nil)
}

func (s *genieService) Start(ctx context.Context, id string) error {
	return s.client.do(ctx, "POST", "/agentic/genies/"+url.PathEscape(id)+"/start", nil, nil)
}

func (s *genieService) Stop(ctx context.Context, id string) error {
	return s.client.do(ctx, "POST", "/agentic/genies/"+url.PathEscape(id)+"/stop", nil, nil)
}

// Assign attaches ids to a genie. The API's 200 doesn't mean every ID was
// applied: unknown knowledge base and user group IDs are silently ignored
// (unknown skill IDs get a 404), so callers must reconcile (ADR-009
// Decision 6).
func (s *genieService) Assign(ctx context.Context, id string, kind GenieAssignment, ids []string) error {
	path := "/agentic/genies/" + url.PathEscape(id) + "/assign_" + string(kind)
	return s.client.do(ctx, "POST", path, map[string][]string{kind.idsField(): ids}, nil)
}

// Remove detaches ids from a genie; IDs that weren't attached are silently
// accepted.
func (s *genieService) Remove(ctx context.Context, id string, kind GenieAssignment, ids []string) error {
	path := "/agentic/genies/" + url.PathEscape(id) + "/remove_" + string(kind)
	return s.client.do(ctx, "POST", path, map[string][]string{kind.idsField(): ids}, nil)
}
