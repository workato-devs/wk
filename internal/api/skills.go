package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type skillService struct {
	client *HTTPClient
}

type skillListResponse struct {
	Data []Skill `json:"data"`
	OffsetPage
}

type skillDataResponse struct {
	Data Skill `json:"data"`
}

func (s *skillService) List(ctx context.Context, opts *PaginationOptions) ([]Skill, OffsetPage, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Page > 0 {
			params.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
	}
	path := "/agentic/skills"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var result skillListResponse
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, OffsetPage{}, err
	}
	return result.Data, result.OffsetPage, nil
}

func (s *skillService) Get(ctx context.Context, id string) (*Skill, error) {
	var result skillDataResponse
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/agentic/skills/%s", id), nil, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (s *skillService) Create(ctx context.Context, recipeID int) (*Skill, error) {
	body := map[string]any{
		"recipe_id": recipeID,
	}
	var result skillDataResponse
	if err := s.client.do(ctx, "POST", "/agentic/skills", body, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}
