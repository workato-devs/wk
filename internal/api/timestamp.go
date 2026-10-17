package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Timestamp is a point in time decoded from whichever format an API resource
// uses, and encoded in one format, so --json callers never handle more than
// one (ADR-009 Decision 6). --raw still shows the wire value.
//
// Observed in the Agent Studio API (2026-10): epoch milliseconds as a number
// (conversations), ISO-8601 in UTC with microseconds (conversation events),
// and ISO-8601 with an offset (genies, skills, knowledge bases).
type Timestamp struct {
	time.Time
}

// UnmarshalJSON accepts a JSON number of epoch milliseconds (or seconds, for
// values too small to be milliseconds since 1973), an RFC 3339 string with
// or without fractional seconds, an ISO-8601 string with no zone (read as
// UTC), a numeric string, or null. 0 and null both mean unset.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		t.Time = time.Time{}
		return nil
	}
	if data[0] != '"' {
		return t.setEpoch(string(data))
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		t.Time = time.Time{}
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
		t.Time = parsed
		return nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02T15:04:05.999999999", s, time.UTC); err == nil {
		t.Time = parsed
		return nil
	}
	if err := t.setEpoch(s); err == nil {
		return nil
	}
	return fmt.Errorf("unrecognized timestamp %q", s)
}

// epochMillisFloor separates epoch seconds from epoch milliseconds: 1e11 ms
// is March 1973, so any real millisecond timestamp is larger.
const epochMillisFloor = 1e11

func (t *Timestamp) setEpoch(s string) error {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return fmt.Errorf("unrecognized timestamp %q", s)
		}
		n = int64(f)
	}
	// 0 means "never" (e.g. a conversation's last_message_at with no
	// messages), not January 1970.
	if n == 0 {
		t.Time = time.Time{}
		return nil
	}
	if n < epochMillisFloor && n > -epochMillisFloor {
		t.Time = time.Unix(n, 0).UTC()
	} else {
		t.Time = time.UnixMilli(n).UTC()
	}
	return nil
}

// MarshalJSON writes RFC 3339 in UTC (with fractional seconds when present),
// or null for an unset time.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.UTC().Format(time.RFC3339Nano))
}

// String formats the time for tables: RFC 3339 in UTC, or "" when unset.
func (t Timestamp) String() string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
