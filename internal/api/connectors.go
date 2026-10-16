package api

import (
	"context"
	"strings"
)

type connectorService struct {
	client *HTTPClient
}

// List returns the workspace's custom SDK connectors.
//
// This calls /custom_connectors. It previously called /integrations, which is a
// different API: a lookup of built-in platform connector metadata by name. With
// no ?applications= parameter /integrations returns {"items": []} and HTTP 200,
// so the command reported an empty workspace instead of failing. With one, it
// returned the named *native* connector (e.g. Salesforce), not a custom one.
//
// search filters client-side, case-insensitively, on name and title. The API's
// own /custom_connectors/search is case-sensitive and returns only the ten most
// recently released connectors, which would silently hide the rest.
func (s *connectorService) List(ctx context.Context, search string) ([]Connector, error) {
	var wrapper struct {
		Result []Connector `json:"result"`
	}
	if err := s.client.do(ctx, "GET", "/custom_connectors", nil, &wrapper); err != nil {
		return nil, err
	}
	if search == "" {
		return wrapper.Result, nil
	}

	needle := strings.ToLower(search)
	var matched []Connector
	for _, c := range wrapper.Result {
		if strings.Contains(strings.ToLower(c.Name), needle) ||
			strings.Contains(strings.ToLower(c.Title), needle) {
			matched = append(matched, c)
		}
	}
	return matched, nil
}
