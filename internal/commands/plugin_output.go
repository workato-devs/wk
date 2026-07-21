package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/workato-devs/wk/internal/plugin"
)

type pluginMethodExecutor interface {
	Execute(pluginName, method string, params any) (json.RawMessage, error)
	Render(pluginName, method string, params plugin.RenderRequest) (string, error)
}

type pluginExecution struct {
	Result       json.RawMessage
	Renderer     string
	RenderedText *string
	RenderErr    error
}

func executePluginMethod(
	executor pluginMethodExecutor,
	pluginName string,
	method string,
	renderer string,
	params any,
	context plugin.RenderContext,
	wantText bool,
) (pluginExecution, error) {
	result, err := executor.Execute(pluginName, method, params)
	if err != nil {
		return pluginExecution{}, err
	}

	execution := pluginExecution{Result: result, Renderer: renderer}
	if !wantText || renderer == "" {
		return execution, nil
	}

	text, err := executor.Render(pluginName, renderer, plugin.RenderRequest{
		Result:  result,
		Context: context,
	})
	if err != nil {
		execution.RenderErr = err
		return execution, nil
	}
	execution.RenderedText = &text
	return execution, nil
}

func writePluginTextResult(w, errW io.Writer, execution pluginExecution) error {
	if execution.RenderedText != nil {
		if _, err := io.WriteString(w, *execution.RenderedText); err != nil {
			return err
		}
		if !strings.HasSuffix(*execution.RenderedText, "\n") {
			_, err := io.WriteString(w, "\n")
			return err
		}
		return nil
	}

	if execution.RenderErr != nil {
		if _, err := fmt.Fprintf(
			errW,
			"warning: plugin renderer %q failed: %v; showing structured result\n",
			execution.Renderer,
			execution.RenderErr,
		); err != nil {
			return err
		}
	}

	return writeStructuredPluginResult(w, execution.Result)
}

// writeStructuredPluginResult is the backward-compatible text fallback for
// plugins without a renderer. Decoding and re-encoding sorts object keys and
// ensures nested values are valid JSON rather than Go map syntax.
func writeStructuredPluginResult(w io.Writer, result json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(result))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		if _, writeErr := w.Write(result); writeErr != nil {
			return writeErr
		}
		if !bytes.HasSuffix(result, []byte("\n")) {
			_, writeErr := io.WriteString(w, "\n")
			return writeErr
		}
		return nil
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
