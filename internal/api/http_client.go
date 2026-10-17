package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/workato-devs/wk/internal/version"
)

// HTTPClient implements the Client interface using HTTP.
type HTTPClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	verbose    bool
	recorder   ResponseRecorder

	recipes        *recipeService
	connections    *connectionService
	folders        *folderService
	packages       *packageService
	tags           *tagService
	apiCollections *apiCollectionService
	apiEndpoints   *apiEndpointService
	apiClients     *apiClientService
	skills         *skillService
	genies         *genieService
	kbs            *knowledgeBaseService
	genieClients   *genieClientService
	guardrails     *guardrailService
	conversations  *conversationService
	mcpServers     *mcpServerService
	workspace      *workspaceService
	connectors     *connectorService
}

// ClientOption configures the HTTPClient.
type ClientOption func(*HTTPClient)

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *HTTPClient) {
		c.httpClient.Timeout = d
	}
}

// WithVerbose enables verbose logging.
func WithVerbose(v bool) ClientOption {
	return func(c *HTTPClient) {
		c.verbose = v
	}
}

// ResponseRecorder receives the body of every successful JSON response, exactly
// as the API sent it. It backs the global --raw flag.
type ResponseRecorder interface {
	RecordResponse(body []byte)
}

// WithResponseRecorder records successful JSON response bodies.
func WithResponseRecorder(r ResponseRecorder) ClientOption {
	return func(c *HTTPClient) {
		c.recorder = r
	}
}

// record passes a successful response body to the recorder, if any. Only JSON
// is recorded, so a binary download (e.g. a package zip) never reaches --raw.
func (c *HTTPClient) record(resp *http.Response, body []byte) {
	if c.recorder == nil || len(body) == 0 {
		return
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return
	}
	c.recorder.RecordResponse(body)
}

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *HTTPClient) {
		c.httpClient = hc
	}
}

// NewHTTPClient creates a new API client.
func NewHTTPClient(baseURL, token string, opts ...ClientOption) *HTTPClient {
	c := &HTTPClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *HTTPClient) Recipes() RecipeService {
	if c.recipes == nil {
		c.recipes = &recipeService{client: c}
	}
	return c.recipes
}

func (c *HTTPClient) Connections() ConnectionService {
	if c.connections == nil {
		c.connections = &connectionService{client: c}
	}
	return c.connections
}

func (c *HTTPClient) Folders() FolderService {
	if c.folders == nil {
		c.folders = &folderService{client: c}
	}
	return c.folders
}

func (c *HTTPClient) Packages() PackageService {
	if c.packages == nil {
		c.packages = &packageService{client: c}
	}
	return c.packages
}

func (c *HTTPClient) Tags() TagService {
	if c.tags == nil {
		c.tags = &tagService{client: c}
	}
	return c.tags
}

func (c *HTTPClient) APICollections() APICollectionService {
	if c.apiCollections == nil {
		c.apiCollections = &apiCollectionService{client: c}
	}
	return c.apiCollections
}

func (c *HTTPClient) APIEndpoints() APIEndpointService {
	if c.apiEndpoints == nil {
		c.apiEndpoints = &apiEndpointService{client: c}
	}
	return c.apiEndpoints
}

func (c *HTTPClient) APIClients() APIClientService {
	if c.apiClients == nil {
		c.apiClients = &apiClientService{client: c}
	}
	return c.apiClients
}

func (c *HTTPClient) Skills() SkillService {
	if c.skills == nil {
		c.skills = &skillService{client: c}
	}
	return c.skills
}

func (c *HTTPClient) Genies() GenieService {
	if c.genies == nil {
		c.genies = &genieService{client: c}
	}
	return c.genies
}

func (c *HTTPClient) KnowledgeBases() KnowledgeBaseService {
	if c.kbs == nil {
		c.kbs = &knowledgeBaseService{client: c}
	}
	return c.kbs
}

func (c *HTTPClient) GenieClients() GenieClientService {
	if c.genieClients == nil {
		c.genieClients = &genieClientService{client: c}
	}
	return c.genieClients
}

func (c *HTTPClient) Guardrails() GuardrailService {
	if c.guardrails == nil {
		c.guardrails = &guardrailService{client: c}
	}
	return c.guardrails
}

func (c *HTTPClient) Conversations() ConversationService {
	if c.conversations == nil {
		c.conversations = &conversationService{client: c}
	}
	return c.conversations
}

func (c *HTTPClient) MCPServers() MCPServerService {
	if c.mcpServers == nil {
		c.mcpServers = &mcpServerService{client: c}
	}
	return c.mcpServers
}

func (c *HTTPClient) Workspace() WorkspaceService {
	if c.workspace == nil {
		c.workspace = &workspaceService{client: c}
	}
	return c.workspace
}

func (c *HTTPClient) Connectors() ConnectorService {
	if c.connectors == nil {
		c.connectors = &connectorService{client: c}
	}
	return c.connectors
}

// do executes an HTTP request and decodes the response.
func (c *HTTPClient) do(ctx context.Context, method, path string, body, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", version.UserAgent())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	if c.verbose {
		fmt.Fprintf(os.Stderr, "[debug] %s %s%s\n", method, c.baseURL, path)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if c.verbose {
		fmt.Fprintf(os.Stderr, "[debug] HTTP %d %s\n", resp.StatusCode, resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
		var errResp struct {
			Message   string `json:"message"`
			Error     string `json:"error"`
			ErrorType string `json:"error_type"`
			Reason    string `json:"reason"`
			// Agent Studio's shape: {"errors":[{"code":400,"title":"…"}]}.
			// 422s add the actual cause as "detail" under a generic title.
			Errors []struct {
				Title  string `json:"title"`
				Detail string `json:"detail"`
			} `json:"errors"`
		}
		if json.Unmarshal(respBody, &errResp) == nil {
			if errResp.Message != "" {
				apiErr.Message = errResp.Message
			} else if errResp.Error != "" {
				apiErr.Message = errResp.Error
			} else if len(errResp.Errors) > 0 && errResp.Errors[0].Title != "" {
				titles := make([]string, 0, len(errResp.Errors))
				for _, e := range errResp.Errors {
					if e.Detail != "" {
						e.Title += ": " + e.Detail
					}
					titles = append(titles, e.Title)
				}
				apiErr.Message = strings.Join(titles, "; ")
			}
			apiErr.ErrorType = errResp.ErrorType
			apiErr.Reason = errResp.Reason
		}
		return apiErr
	}

	c.record(resp, respBody)
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// doRaw executes an HTTP request and returns the raw response body.
func (c *HTTPClient) doRaw(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", version.UserAgent())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.verbose {
		fmt.Fprintf(os.Stderr, "[debug] %s %s%s\n", method, c.baseURL, path)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if c.verbose {
		fmt.Fprintf(os.Stderr, "[debug] HTTP %d %s\n", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(data),
		}
	}
	c.record(resp, data)
	return data, nil
}
