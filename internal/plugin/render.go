package plugin

import (
	"encoding/json"
	"fmt"
)

const RenderFormatText = "text"

// RenderContext describes the CLI output request. Fields may be added over
// time; renderers should ignore context fields they do not recognize.
type RenderContext struct {
	Format      string `json:"format"`
	CommandPath string `json:"command_path,omitempty"`
}

// RenderRequest asks a plugin to present an already-computed command result.
// Rendering is a presentation-only operation and must not rerun the command.
type RenderRequest struct {
	Result  json.RawMessage `json:"result"`
	Context RenderContext   `json:"context"`
}

// RenderResponse is the result envelope returned by a plugin renderer.
type RenderResponse struct {
	Text string `json:"text"`
}

// DecodeRenderResponse validates and extracts a renderer response. An empty
// text value is valid, but the text field itself is required.
func DecodeRenderResponse(result json.RawMessage) (string, error) {
	var response struct {
		Text *string `json:"text"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return "", fmt.Errorf("decoding renderer response: %w", err)
	}
	if response.Text == nil {
		return "", fmt.Errorf("renderer response is missing required text field")
	}
	return *response.Text, nil
}
