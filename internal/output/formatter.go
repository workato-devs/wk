package output

import "io"

// Formatter defines how command output is rendered.
type Formatter interface {
	// Format writes a single data object.
	Format(w io.Writer, data any) error
	// FormatList writes tabular data with headers and rows.
	FormatList(w io.Writer, headers []string, rows [][]string) error
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
