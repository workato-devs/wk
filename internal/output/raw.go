package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// ResponseBuffer collects API response bodies for --raw. It satisfies
// api.ResponseRecorder.
type ResponseBuffer struct {
	mu     sync.Mutex
	bodies [][]byte
}

// RecordResponse stores a copy of body.
func (b *ResponseBuffer) RecordResponse(body []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bodies = append(b.bodies, append([]byte(nil), body...))
}

// Bodies returns the recorded bodies in request order.
func (b *ResponseBuffer) Bodies() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.bodies
}

// RawFormatter prints what the Workato API returned instead of wk's own
// rendering (ADR-001 Decision 3, ADR-009 Decision 7): --json is wk's modeled
// contract, --raw is the API's. One response is written byte for byte. Several
// (a command that made more than one request) are written one per line, in
// request order, with only insignificant whitespace removed. A command that
// made no API request falls back to wk's JSON, with a note on stderr.
type RawFormatter struct {
	Responses *ResponseBuffer
	fallback  JSONFormatter
}

func (f *RawFormatter) Format(w io.Writer, data any) error {
	bodies := f.Responses.Bodies()
	switch len(bodies) {
	case 0:
		fmt.Fprintln(os.Stderr, "--raw: this command made no Workato API request, so this is wk's JSON")
		return f.fallback.Format(w, data)
	case 1:
		body := bodies[0]
		if _, err := w.Write(body); err != nil {
			return err
		}
		if !bytes.HasSuffix(body, []byte("\n")) {
			_, err := w.Write([]byte("\n"))
			return err
		}
		return nil
	default:
		for _, body := range bodies {
			var line bytes.Buffer
			if json.Compact(&line, body) != nil {
				line.Reset()
				line.Write(bytes.TrimSpace(body))
			}
			line.WriteByte('\n')
			if _, err := w.Write(line.Bytes()); err != nil {
				return err
			}
		}
		return nil
	}
}

func (f *RawFormatter) FormatList(w io.Writer, data any, _ []string, _ [][]string) error {
	return f.Format(w, data)
}

func (f *RawFormatter) FormatPage(w io.Writer, data any, _ []string, _ [][]string, _ PageMeta) error {
	return f.Format(w, data)
}

func (f *RawFormatter) FormatCursorPage(w io.Writer, data any, _ []string, _ [][]string, _ CursorMeta) error {
	return f.Format(w, data)
}
