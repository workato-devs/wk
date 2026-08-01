package plugin

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeRenderResponse(t *testing.T) {
	tests := []struct {
		name    string
		result  json.RawMessage
		want    string
		wantErr string
	}{
		{name: "text", result: json.RawMessage(`{"text":"hello"}`), want: "hello"},
		{name: "empty text", result: json.RawMessage(`{"text":""}`), want: ""},
		{name: "extra fields", result: json.RawMessage(`{"text":"hello","style":"plain"}`), want: "hello"},
		{name: "missing text", result: json.RawMessage(`{"message":"hello"}`), wantErr: "missing required text field"},
		{name: "wrong text type", result: json.RawMessage(`{"text":42}`), wantErr: "decoding renderer response"},
		{name: "not an object", result: json.RawMessage(`"hello"`), wantErr: "decoding renderer response"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeRenderResponse(tt.result)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("DecodeRenderResponse() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeRenderResponse() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("DecodeRenderResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderRequestJSONContract(t *testing.T) {
	request := RenderRequest{
		Result: json.RawMessage(`{"exit_code":1,"files":[]}`),
		Context: RenderContext{
			Format:      RenderFormatText,
			CommandPath: "wk lint",
		},
	}

	got, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(RenderRequest): %v", err)
	}
	want := `{"result":{"exit_code":1,"files":[]},"context":{"format":"text","command_path":"wk lint"}}`
	if string(got) != want {
		t.Errorf("RenderRequest JSON = %s, want %s", got, want)
	}
}
