package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONFormat(t *testing.T) {
	f := &JSONFormatter{}
	var buf bytes.Buffer

	data := map[string]string{"name": "test", "version": "1.0"}
	if err := f.Format(&buf, data); err != nil {
		t.Fatalf("Format: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("name = %q, want %q", result["name"], "test")
	}
}

func TestJSONFormatList(t *testing.T) {
	f := &JSONFormatter{}
	var buf bytes.Buffer

	headers := []string{"id", "name"}
	rows := [][]string{{"1", "alpha"}, {"2", "beta"}}

	if err := f.FormatList(&buf, headers, rows); err != nil {
		t.Fatalf("FormatList: %v", err)
	}

	var result []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0]["name"] != "alpha" {
		t.Errorf("result[0][name] = %q, want %q", result[0]["name"], "alpha")
	}
}

func TestTextFormatList(t *testing.T) {
	f := &TextFormatter{}
	var buf bytes.Buffer

	headers := []string{"ID", "NAME"}
	rows := [][]string{{"1", "alpha"}, {"2", "beta"}}

	if err := f.FormatList(&buf, headers, rows); err != nil {
		t.Fatalf("FormatList: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ID") || !strings.Contains(output, "NAME") {
		t.Errorf("output missing headers: %s", output)
	}
	if !strings.Contains(output, "alpha") || !strings.Contains(output, "beta") {
		t.Errorf("output missing data: %s", output)
	}
}

func TestJSONFormatStructSlice(t *testing.T) {
	type item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	f := &JSONFormatter{}
	var buf bytes.Buffer

	items := []item{{ID: 42, Name: "alpha"}, {ID: 7, Name: "beta"}}
	if err := f.Format(&buf, items); err != nil {
		t.Fatalf("Format: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	// Verify lowercase json tag keys are used
	if _, ok := result[0]["id"]; !ok {
		t.Error("expected lowercase key 'id'")
	}
	if _, ok := result[0]["ID"]; ok {
		t.Error("unexpected uppercase key 'ID'")
	}
	// Verify numeric types are preserved (json.Number or float64)
	idVal, ok := result[0]["id"].(float64)
	if !ok {
		t.Fatalf("id should be numeric, got %T", result[0]["id"])
	}
	if idVal != 42 {
		t.Errorf("id = %v, want 42", idVal)
	}
	if result[0]["name"] != "alpha" {
		t.Errorf("name = %v, want alpha", result[0]["name"])
	}
}

func TestNewFormatter(t *testing.T) {
	jf := NewFormatter("json")
	if _, ok := jf.(*JSONFormatter); !ok {
		t.Error("NewFormatter(json) should return JSONFormatter")
	}

	tf := NewFormatter("text")
	if _, ok := tf.(*TextFormatter); !ok {
		t.Error("NewFormatter(text) should return TextFormatter")
	}

	df := NewFormatter("")
	if _, ok := df.(*TextFormatter); !ok {
		t.Error("NewFormatter('') should default to TextFormatter")
	}
}
