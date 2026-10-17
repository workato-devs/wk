package api

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

// The three formats the Agent Studio API uses all decode to the same instant
// and encode to one format.
func TestTimestamp_AgentStudioFormats(t *testing.T) {
	want := time.Date(2026, 5, 28, 19, 39, 38, 427000000, time.UTC)
	for name, raw := range map[string]string{
		"epoch millis (conversations)":          `1779997178427`,
		"ISO UTC with micros (events)":          `"2026-05-28T19:39:38.427000Z"`,
		"ISO with offset (genies, skills, KBs)": `"2026-05-28T12:39:38.427-07:00"`,
		"numeric string":                        `"1779997178427"`,
		"no zone, read as UTC":                  `"2026-05-28T19:39:38.427"`,
	} {
		t.Run(name, func(t *testing.T) {
			var ts Timestamp
			if err := json.Unmarshal([]byte(raw), &ts); err != nil {
				t.Fatalf("unmarshal %s: %v", raw, err)
			}
			if !ts.Equal(want) {
				t.Errorf("%s = %s, want %s", raw, ts.UTC(), want)
			}
			out, _ := json.Marshal(ts)
			if string(out) != `"2026-05-28T19:39:38.427Z"` {
				t.Errorf("marshal = %s, want RFC 3339 in UTC", out)
			}
		})
	}
}

func TestTimestamp_NullAndInvalid(t *testing.T) {
	var ts Timestamp
	if err := json.Unmarshal([]byte(`null`), &ts); err != nil || !ts.IsZero() {
		t.Errorf("null: %v, %v", ts, err)
	}
	if out, _ := json.Marshal(ts); string(out) != "null" {
		t.Errorf("zero marshals as %s, want null", out)
	}
	if err := json.Unmarshal([]byte(`"yesterday"`), &ts); err == nil {
		t.Error("want an error for an unrecognized timestamp")
	}
	// last_message_at is 0 for a conversation with no messages: unset, not 1970.
	if err := json.Unmarshal([]byte(`0`), &ts); err != nil || !ts.IsZero() {
		t.Errorf("0 decoded as %s, %v; want unset", ts, err)
	}
	// Epoch seconds aren't misread as 1970 milliseconds.
	if err := json.Unmarshal([]byte(`1779997178`), &ts); err != nil || ts.Year() != 2026 {
		t.Errorf("epoch seconds decoded as %s, %v", ts, err)
	}
}

func TestCursor(t *testing.T) {
	q := url.Values{}
	(&CursorOptions{After: "abc", Size: 50}).apply(q)
	if q.Get("page[after]") != "abc" || q.Get("page[size]") != "50" {
		t.Errorf("query = %v", q)
	}
	(*CursorOptions)(nil).apply(url.Values{}) // nil is the first page

	for raw, wantNext := range map[string]bool{
		`{"data":[],"next_cursor":null}`:     false, // an empty page sends null
		`{"data":[{}],"next_cursor":""}`:     false,
		`{"data":[{}]}`:                      false,
		`{"data":[{}],"next_cursor":"eyJ9"}`: true,
	} {
		var env cursorEnvelope[map[string]any]
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			t.Fatal(err)
		}
		if got := env.page().HasNext; got != wantNext {
			t.Errorf("%s: HasNext = %v, want %v", raw, got, wantNext)
		}
	}
}
