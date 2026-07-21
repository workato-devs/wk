package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/workato-devs/wk/internal/plugin"
)

type fakePluginExecutor struct {
	executeResult json.RawMessage
	executeErr    error
	renderText    string
	renderErr     error
	calls         []string
	renderRequest plugin.RenderRequest
}

func (f *fakePluginExecutor) Execute(pluginName, method string, params any) (json.RawMessage, error) {
	f.calls = append(f.calls, pluginName+"/"+method)
	return f.executeResult, f.executeErr
}

func (f *fakePluginExecutor) Render(pluginName, method string, params plugin.RenderRequest) (string, error) {
	f.calls = append(f.calls, pluginName+"/"+method)
	f.renderRequest = params
	return f.renderText, f.renderErr
}

func TestExecutePluginMethod_TextCallsRendererWithPrimaryResult(t *testing.T) {
	result := json.RawMessage(`{"exit_code":1,"files":[{"file":"recipe.json"}]}`)
	executor := &fakePluginExecutor{executeResult: result, renderText: "1 error"}
	context := plugin.RenderContext{Format: plugin.RenderFormatText, CommandPath: "wk lint"}

	execution, err := executePluginMethod(
		executor,
		"recipe-lint",
		"lint.run",
		"lint.render",
		map[string]any{"files": []string{"recipe.json"}},
		context,
		true,
	)
	if err != nil {
		t.Fatalf("executePluginMethod: %v", err)
	}

	wantCalls := []string{"recipe-lint/lint.run", "recipe-lint/lint.render"}
	if !reflect.DeepEqual(executor.calls, wantCalls) {
		t.Errorf("calls = %v, want %v", executor.calls, wantCalls)
	}
	if !bytes.Equal(executor.renderRequest.Result, result) {
		t.Errorf("renderer result = %s, want %s", executor.renderRequest.Result, result)
	}
	if executor.renderRequest.Context != context {
		t.Errorf("renderer context = %#v, want %#v", executor.renderRequest.Context, context)
	}
	if execution.RenderedText == nil || *execution.RenderedText != "1 error" {
		t.Errorf("RenderedText = %v, want %q", execution.RenderedText, "1 error")
	}
}

func TestExecutePluginMethod_JSONSkipsRenderer(t *testing.T) {
	executor := &fakePluginExecutor{executeResult: json.RawMessage(`{"ok":true}`)}

	_, err := executePluginMethod(
		executor,
		"example",
		"example.run",
		"example.render",
		nil,
		plugin.RenderContext{Format: plugin.RenderFormatText},
		false,
	)
	if err != nil {
		t.Fatalf("executePluginMethod: %v", err)
	}

	wantCalls := []string{"example/example.run"}
	if !reflect.DeepEqual(executor.calls, wantCalls) {
		t.Errorf("calls = %v, want %v", executor.calls, wantCalls)
	}
}

func TestExecutePluginMethod_MissingRendererSkipsSecondCall(t *testing.T) {
	executor := &fakePluginExecutor{executeResult: json.RawMessage(`{"ok":true}`)}

	execution, err := executePluginMethod(
		executor,
		"legacy",
		"legacy.run",
		"",
		nil,
		plugin.RenderContext{Format: plugin.RenderFormatText},
		true,
	)
	if err != nil {
		t.Fatalf("executePluginMethod: %v", err)
	}
	if len(executor.calls) != 1 {
		t.Errorf("calls = %v, want only primary method", executor.calls)
	}
	if execution.RenderedText != nil || execution.RenderErr != nil {
		t.Errorf("execution = %#v, want fallback without renderer error", execution)
	}
}

func TestExecutePluginMethod_RendererFailureIsNonFatal(t *testing.T) {
	renderErr := errors.New("render failed")
	executor := &fakePluginExecutor{
		executeResult: json.RawMessage(`{"exit_code":1}`),
		renderErr:     renderErr,
	}

	execution, err := executePluginMethod(
		executor,
		"recipe-lint",
		"lint.run",
		"lint.render",
		nil,
		plugin.RenderContext{Format: plugin.RenderFormatText},
		true,
	)
	if err != nil {
		t.Fatalf("executePluginMethod: %v", err)
	}
	if !errors.Is(execution.RenderErr, renderErr) {
		t.Errorf("RenderErr = %v, want %v", execution.RenderErr, renderErr)
	}
	if execution.RenderedText != nil {
		t.Errorf("RenderedText = %q, want nil", *execution.RenderedText)
	}
}

func TestExecutePluginMethod_PrimaryFailureSkipsRenderer(t *testing.T) {
	executor := &fakePluginExecutor{executeErr: errors.New("command failed")}

	_, err := executePluginMethod(
		executor,
		"example",
		"example.run",
		"example.render",
		nil,
		plugin.RenderContext{Format: plugin.RenderFormatText},
		true,
	)
	if err == nil || err.Error() != "command failed" {
		t.Fatalf("executePluginMethod error = %v, want command failed", err)
	}
	if len(executor.calls) != 1 {
		t.Errorf("calls = %v, want only primary method", executor.calls)
	}
}

func TestWritePluginTextResult_RenderedText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "adds newline", text: "hello", want: "hello\n"},
		{name: "preserves newline", text: "hello\n", want: "hello\n"},
		{name: "empty is valid", text: "", want: "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			execution := pluginExecution{RenderedText: &tt.text}
			if err := writePluginTextResult(&stdout, &stderr, execution); err != nil {
				t.Fatalf("writePluginTextResult: %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestWritePluginTextResult_MissingRendererUsesDeterministicJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	execution := pluginExecution{
		Result: json.RawMessage(`{"z":{"items":[{"b":2,"a":1}]},"a":0}`),
	}

	if err := writePluginTextResult(&stdout, &stderr, execution); err != nil {
		t.Fatalf("writePluginTextResult: %v", err)
	}

	want := "{\n  \"a\": 0,\n  \"z\": {\n    \"items\": [\n      {\n        \"a\": 1,\n        \"b\": 2\n      }\n    ]\n  }\n}\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout =\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(stdout.String(), "map[") {
		t.Errorf("stdout contains Go map syntax: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestWritePluginTextResult_RendererFailureFallsBackAndPreservesExitCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	execution := pluginExecution{
		Result:    json.RawMessage(`{"exit_code":2,"message":"invalid"}`),
		Renderer:  "lint.render",
		RenderErr: errors.New("rpc error -32603: render failed"),
	}

	if err := writePluginTextResult(&stdout, &stderr, execution); err != nil {
		t.Fatalf("writePluginTextResult: %v", err)
	}
	if !strings.Contains(stderr.String(), `plugin renderer "lint.render" failed`) {
		t.Errorf("stderr = %q, want renderer warning", stderr.String())
	}
	if strings.Contains(stdout.String(), "map[") {
		t.Errorf("stdout contains Go map syntax: %s", stdout.String())
	}

	err := exitCodeFromResult(execution.Result)
	var exitErr ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Errorf("exitCodeFromResult() = %v, want ExitCodeError{Code: 2}", err)
	}
}
