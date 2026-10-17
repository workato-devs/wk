package output

import "io"

// Formatter defines how command output is rendered.
type Formatter interface {
	// Format writes a single data object.
	Format(w io.Writer, data any) error
	// FormatList writes a list. In JSON mode, data (the native structs) is
	// encoded as-is; in text mode headers+rows render as a table. JSON never
	// comes from the rows: those hold display values (truncated hashes,
	// "(unset)", "*") under display headers, which is not a contract (#102).
	FormatList(w io.Writer, data any, headers []string, rows [][]string) error
	// FormatPage writes a paginated list response. In JSON mode, data
	// (the native API structs) is wrapped with pagination metadata; in
	// text mode it renders headers+rows as a table. This preserves native
	// field names, types, and omitted fields in JSON output.
	FormatPage(w io.Writer, data any, headers []string, rows [][]string, meta PageMeta) error
	// FormatCursorPage is FormatPage for cursor-paginated lists: JSON wraps
	// data with next_cursor/has_next instead of page/per_page.
	FormatCursorPage(w io.Writer, data any, headers []string, rows [][]string, meta CursorMeta) error
}

// CursorMeta is where a cursor-paginated list stands. HasNext comes from the
// server's cursor, never from the page length (ADR-009 Decision 2).
type CursorMeta struct {
	NextCursor string `json:"next_cursor"`
	HasNext    bool   `json:"has_next"`
}

// PageMeta carries pagination context so agents can iterate through
// result sets without parsing text or guessing when to stop.
type PageMeta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	HasNext bool `json:"has_next"`
}

// NewFormatter returns a Formatter for the given format string.
// Supported: "json", "text" (default).
func NewFormatter(format string) Formatter {
	switch format {
	case "json":
		return &JSONFormatter{}
	default:
		return &TextFormatter{}
	}
}
