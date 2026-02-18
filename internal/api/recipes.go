package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
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
		if opts.Status == "running" {
			params.Set("active", "true")
		} else if opts.Status == "stopped" {
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
	return s.client.do(ctx, "PUT", fmt.Sprintf("/recipes/%d/start", id), nil, nil)
}

func (s *recipeService) Stop(ctx context.Context, id int) error {
	return s.client.do(ctx, "PUT", fmt.Sprintf("/recipes/%d/stop", id), nil, nil)
}

func (s *recipeService) Export(ctx context.Context, id int) ([]byte, error) {
	return s.client.doRaw(ctx, "GET", fmt.Sprintf("/recipes/%d", id))
}

func (s *recipeService) Import(ctx context.Context, folderID int, data []byte) (*Recipe, error) {
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("invalid recipe JSON: %w", err)
	}
	body["folder_id"] = strconv.Itoa(folderID)

	// The Workato API expects "code" and "config" as JSON-encoded strings,
	// but recipe exports return them as objects. Stringify them for import.
	for _, key := range []string{"code", "config"} {
		if v, ok := body[key]; ok {
			if _, isString := v.(string); !isString {
				encoded, err := json.Marshal(v)
				if err != nil {
					return nil, fmt.Errorf("encoding %s: %w", key, err)
				}
				body[key] = string(encoded)
			}
		}
	}

	var recipe Recipe
	if err := s.client.do(ctx, "POST", "/recipes", body, &recipe); err != nil {
		return nil, err
	}
	return &recipe, nil
}

