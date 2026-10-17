package output

import (
	"encoding/json"
	"io"
	"reflect"
)

// JSONFormatter outputs data as pretty-printed JSON.
type JSONFormatter struct{}

func (f *JSONFormatter) Format(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func (f *JSONFormatter) FormatList(w io.Writer, data any, _ []string, _ [][]string) error {
	// A list is always an array, even when empty: a nil slice would encode
	// as null and break callers that iterate the result.
	if v := reflect.ValueOf(data); v.Kind() == reflect.Slice && v.IsNil() {
		data = []any{}
	}
	return f.Format(w, data)
}

func (f *JSONFormatter) FormatPage(w io.Writer, data any, _ []string, _ [][]string, meta PageMeta) error {
	envelope := struct {
		Items   any  `json:"items"`
		Page    int  `json:"page"`
		PerPage int  `json:"per_page"`
		HasNext bool `json:"has_next"`
	}{
		Items:   data,
		Page:    meta.Page,
		PerPage: meta.PerPage,
		HasNext: meta.HasNext,
	}
	return f.Format(w, envelope)
}

func (f *JSONFormatter) FormatCursorPage(w io.Writer, data any, _ []string, _ [][]string, meta CursorMeta) error {
	if v := reflect.ValueOf(data); v.Kind() == reflect.Slice && v.IsNil() {
		data = []any{}
	}
	envelope := struct {
		Items      any    `json:"items"`
		NextCursor string `json:"next_cursor"`
		HasNext    bool   `json:"has_next"`
	}{Items: data, NextCursor: meta.NextCursor, HasNext: meta.HasNext}
	return f.Format(w, envelope)
}
