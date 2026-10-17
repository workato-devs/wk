package api

import (
	"net/url"
	"strconv"
)

// CursorOptions pages through a cursor-paginated endpoint. It is a separate
// type from PaginationOptions on purpose (ADR-009 Decision 2): the two styles
// aren't variants of one thing, and keeping them apart makes `--page` against
// a cursor endpoint impossible to write.
type CursorOptions struct {
	After string // the previous page's NextCursor; empty for the first page
	Size  int    // page size; 0 leaves the server default
}

// apply adds the cursor parameters Agent Studio uses (page[after],
// page[size]) to q.
func (o *CursorOptions) apply(q url.Values) {
	if o == nil {
		return
	}
	if o.After != "" {
		q.Set("page[after]", o.After)
	}
	if o.Size > 0 {
		q.Set("page[size]", strconv.Itoa(o.Size))
	}
}

// CursorPage describes where a cursor-paginated result stands.
type CursorPage struct {
	NextCursor string
	// HasNext is true when the server returned a cursor to continue from.
	// Never inferred from the page length, which is what produces the
	// `len(items) == perPage` class of bug.
	HasNext bool
}

// cursorEnvelope is the {"data": [...], "next_cursor": ...} shape of a cursor
// page. next_cursor is a pointer because the server sends null when there is
// no next page (an empty page returns {"data":[],"next_cursor":null}).
type cursorEnvelope[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

func (e cursorEnvelope[T]) page() CursorPage {
	if e.NextCursor == nil || *e.NextCursor == "" {
		return CursorPage{}
	}
	return CursorPage{NextCursor: *e.NextCursor, HasNext: true}
}
