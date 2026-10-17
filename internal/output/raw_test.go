package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRawFormatter(t *testing.T) {
	t.Run("one response is written byte for byte", func(t *testing.T) {
		buf := &ResponseBuffer{}
		buf.RecordResponse([]byte(`{"b": 1,  "a":[2]}`))
		var out bytes.Buffer
		if err := (&RawFormatter{Responses: buf}).FormatList(&out, []string{"ignored"}, nil, nil); err != nil {
			t.Fatal(err)
		}
		if out.String() != `{"b": 1,  "a":[2]}`+"\n" {
			t.Errorf("out = %q, want the body verbatim", out.String())
		}
	})
	t.Run("several responses are one per line, in order", func(t *testing.T) {
		buf := &ResponseBuffer{}
		buf.RecordResponse([]byte("{\n  \"page\": 1\n}"))
		buf.RecordResponse([]byte(`[1, 2]`))
		var out bytes.Buffer
		(&RawFormatter{Responses: buf}).Format(&out, nil)
		if out.String() != "{\"page\":1}\n[1,2]\n" {
			t.Errorf("out = %q", out.String())
		}
	})
	t.Run("no API response falls back to wk's JSON", func(t *testing.T) {
		var out bytes.Buffer
		(&RawFormatter{Responses: &ResponseBuffer{}}).Format(&out, map[string]string{"k": "v"})
		if !strings.Contains(out.String(), `"k": "v"`) {
			t.Errorf("out = %q, want the fallback JSON", out.String())
		}
	})
}
