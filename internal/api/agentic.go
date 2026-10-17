package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// Agent Studio read services (ADR-009 Decision 1). Shapes are in types.go.

// KBListOptions filters and pages GET /agentic/knowledge_bases.
type KBListOptions struct {
	FolderID  string
	ProjectID string
	Query     string
	Page      int
	PerPage   int // max 50
}

type knowledgeBaseService struct{ client *HTTPClient }

func (s *knowledgeBaseService) List(ctx context.Context, opts *KBListOptions) ([]KnowledgeBase, OffsetPage, error) {
	params := url.Values{}
	if opts != nil {
		setIf(params, "folder_id", opts.FolderID)
		setIf(params, "project_id", opts.ProjectID)
		setIf(params, "query", opts.Query)
		setPage(params, opts.Page, opts.PerPage)
	}
	var result struct {
		Data []KnowledgeBase `json:"data"`
		OffsetPage
	}
	if err := s.client.do(ctx, "GET", withQuery("/agentic/knowledge_bases", params), nil, &result); err != nil {
		return nil, OffsetPage{}, err
	}
	return result.Data, result.OffsetPage, nil
}

func (s *knowledgeBaseService) Get(ctx context.Context, id string) (*KnowledgeBase, error) {
	var result struct {
		Data KnowledgeBase `json:"data"`
	}
	if err := s.client.do(ctx, "GET", "/agentic/knowledge_bases/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// Recipes lists a knowledge base's recipes. Only empty lists have been
// observed, so each element is kept as sent.
func (s *knowledgeBaseService) Recipes(ctx context.Context, id string, opts *PaginationOptions) ([]json.RawMessage, OffsetPage, error) {
	params := url.Values{}
	if opts != nil {
		setPage(params, opts.Page, opts.PerPage)
	}
	var result struct {
		Data []json.RawMessage `json:"data"`
		OffsetPage
	}
	path := withQuery("/agentic/knowledge_bases/"+url.PathEscape(id)+"/recipes", params)
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, OffsetPage{}, err
	}
	return result.Data, result.OffsetPage, nil
}

// DataSources returns a Matrix knowledge base's data sources, kept as sent:
// no populated response has been observed. Recipe knowledge bases answer 400.
func (s *knowledgeBaseService) DataSources(ctx context.Context, id string) (json.RawMessage, error) {
	var result struct {
		Data json.RawMessage `json:"data"`
	}
	if err := s.client.do(ctx, "GET", "/agentic/knowledge_bases/"+url.PathEscape(id)+"/data_sources", nil, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

type genieClientService struct{ client *HTTPClient }

// List returns one page of genie clients. The reference documents no
// parameters, but responses carry page/per_page/total and honor them.
func (s *genieClientService) List(ctx context.Context, opts *PaginationOptions) ([]GenieClient, OffsetPage, error) {
	params := url.Values{}
	if opts != nil {
		setPage(params, opts.Page, opts.PerPage)
	}
	var result struct {
		Data []GenieClient `json:"data"`
		OffsetPage
	}
	if err := s.client.do(ctx, "GET", withQuery("/agentic/genies/clients", params), nil, &result); err != nil {
		return nil, OffsetPage{}, err
	}
	return result.Data, result.OffsetPage, nil
}

type guardrailService struct{ client *HTTPClient }

func (s *guardrailService) Get(ctx context.Context, genieID string) (*Guardrail, error) {
	var result struct {
		Data Guardrail `json:"data"`
	}
	if err := s.client.do(ctx, "GET", "/agentic/genies/"+url.PathEscape(genieID)+"/guardrails", nil, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// PIIEntities lists the PII entity types a pii_detection policy can use.
func (s *guardrailService) PIIEntities(ctx context.Context) ([]string, error) {
	var result struct {
		Data []string `json:"data"`
	}
	if err := s.client.do(ctx, "GET", "/agentic/genies/guardrails/policies/pii_entities", nil, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// ConversationListOptions filters and pages a genie's conversations.
type ConversationListOptions struct {
	Cursor CursorOptions // page size max 100
	From   string        // YYYY-MM-DDTHH:mm:ss.SSSZ
	To     string
}

// EventListOptions filters and pages a conversation's events.
type EventListOptions struct {
	Cursor     CursorOptions // page size max 100
	Order      string
	EventTypes []string
	GenieRunID string
}

type conversationService struct{ client *HTTPClient }

func (s *conversationService) List(ctx context.Context, genieID string, opts *ConversationListOptions) ([]Conversation, CursorPage, error) {
	params := url.Values{}
	if opts != nil {
		opts.Cursor.apply(params)
		setIf(params, "from", opts.From)
		setIf(params, "to", opts.To)
	}
	var env cursorEnvelope[Conversation]
	path := withQuery("/agentic/genies/"+url.PathEscape(genieID)+"/conversations", params)
	if err := s.client.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, CursorPage{}, err
	}
	return env.Data, env.page(), nil
}

func (s *conversationService) Get(ctx context.Context, genieID, conversationID string) (*ConversationDetail, error) {
	var result struct {
		Data ConversationDetail `json:"data"`
	}
	path := fmt.Sprintf("/agentic/genies/%s/conversations/%s", url.PathEscape(genieID), url.PathEscape(conversationID))
	if err := s.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (s *conversationService) Events(ctx context.Context, genieID, conversationID string, opts *EventListOptions) ([]ConversationEvent, CursorPage, error) {
	params := url.Values{}
	if opts != nil {
		opts.Cursor.apply(params)
		setIf(params, "order", opts.Order)
		setIf(params, "genie_run_id", opts.GenieRunID)
		for _, t := range opts.EventTypes {
			params.Add("include_event_types[]", t)
		}
	}
	var env cursorEnvelope[ConversationEvent]
	path := withQuery(fmt.Sprintf("/agentic/genies/%s/conversations/%s/events", url.PathEscape(genieID), url.PathEscape(conversationID)), params)
	if err := s.client.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, CursorPage{}, err
	}
	return env.Data, env.page(), nil
}

func setIf(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func setPage(q url.Values, page, perPage int) {
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		q.Set("per_page", strconv.Itoa(perPage))
	}
}

func withQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}
