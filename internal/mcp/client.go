package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/version"
)

// Client is an MCP protocol client using Streamable HTTP transport.
type Client struct {
	url        string
	httpClient *http.Client
	sessionID  string
	bearer     string
}

// NewClient creates a new MCP client for the given server URL.
func NewClient(url string) *Client {
	return &Client{
		url:        url,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// WithBearer makes the client send token as an Authorization bearer header,
// for first-party endpoints such as airo_mcp that take a header rather than a
// URL-embedded credential (ADR-008 Decision 3). Callers decide which hosts may
// receive it.
func (c *Client) WithBearer(token string) *Client {
	c.bearer = token
	return c
}

// HTTPError is a non-2xx response from an MCP server. It keeps the body and
// any WWW-Authenticate challenge, which say why (e.g. the OAuth discovery
// document for a 401).
type HTTPError struct {
	StatusCode      int
	Status          string
	Body            string
	WWWAuthenticate string
}

func (e *HTTPError) Error() string {
	msg := "HTTP " + e.Status
	if e.Body != "" {
		msg += ": " + e.Body
	}
	if e.WWWAuthenticate != "" {
		msg += " (WWW-Authenticate: " + e.WWWAuthenticate + ")"
	}
	return msg
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	ID      int    `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Initialize performs the MCP initialize handshake.
func (c *Client) Initialize(ctx context.Context) (*api.MCPServerInfo, error) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "wk",
				"version": version.Version(),
			},
		},
		ID: 1,
	}

	resp, err := c.send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}

	var result struct {
		ProtocolVersion string         `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Capabilities map[string]any `json:"capabilities"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parsing initialize result: %w", err)
	}

	// The MCP lifecycle requires this notification before any other request.
	if err := c.notify(ctx, "notifications/initialized"); err != nil {
		return nil, fmt.Errorf("notifications/initialized: %w", err)
	}

	return &api.MCPServerInfo{
		Name:            result.ServerInfo.Name,
		Version:         result.ServerInfo.Version,
		ProtocolVersion: result.ProtocolVersion,
		Capabilities:    result.Capabilities,
	}, nil
}

// ListTools retrieves the list of tools from the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]api.MCPTool, error) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		Params:  map[string]any{},
		ID:      2,
	}

	resp, err := c.send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}

	var result struct {
		Tools []api.MCPTool `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parsing tools/list result: %w", err)
	}

	return result.Tools, nil
}

// notify sends a JSON-RPC notification (no id, no response expected).
func (c *Client) notify(ctx context.Context, method string) error {
	resp, err := c.post(ctx, map[string]any{"jsonrpc": "2.0", "method": method})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// post sends a JSON-RPC message and returns the response for any 2xx status;
// non-2xx statuses become an *HTTPError.
func (c *Client) post(ctx context.Context, msg any) (*http.Response, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json")
	req.Header.Set("User-Agent", version.UserAgent())
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, &HTTPError{
			StatusCode:      resp.StatusCode,
			Status:          resp.Status,
			Body:            strings.TrimSpace(string(snippet)),
			WWWAuthenticate: resp.Header.Get("WWW-Authenticate"),
		}
	}
	return resp, nil
}

func (c *Client) send(ctx context.Context, rpcReq jsonRPCRequest) (*jsonRPCResponse, error) {
	resp, err := c.post(ctx, rpcReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")

	// Handle SSE response
	if strings.Contains(contentType, "text/event-stream") {
		return parseSSE(resp.Body)
	}

	// Handle plain JSON response
	var rpcResp jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decoding JSON-RPC response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return &rpcResp, nil
}

func parseSSE(body interface{ Read([]byte) (int, error) }) (*jsonRPCResponse, error) {
	scanner := bufio.NewScanner(body)
	var dataLine string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading SSE: %w", err)
	}

	if dataLine == "" {
		return nil, fmt.Errorf("no data in SSE response")
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal([]byte(dataLine), &rpcResp); err != nil {
		return nil, fmt.Errorf("parsing SSE data: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return &rpcResp, nil
}
