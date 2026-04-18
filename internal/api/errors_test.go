package api

import (
	"errors"
	"fmt"
	"testing"

	wkerrors "github.com/workato-devs/wk-cli-beta/internal/errors"
)

func TestAPIError_Is(t *testing.T) {
	tests := []struct {
		name   string
		status int
		target error
		want   bool
	}{
		{"401 matches unauthorized", 401, wkerrors.ErrAPIUnauthorized, true},
		{"403 matches forbidden", 403, wkerrors.ErrAPIForbidden, true},
		{"404 matches not found", 404, wkerrors.ErrAPINotFound, true},
		{"429 matches rate limit", 429, wkerrors.ErrAPIRateLimit, true},
		{"500 matches server", 500, wkerrors.ErrAPIServer, true},
		{"503 matches server", 503, wkerrors.ErrAPIServer, true},
		{"404 does not match unauthorized", 404, wkerrors.ErrAPIUnauthorized, false},
		{"200 matches nothing", 200, wkerrors.ErrAPINotFound, false},
		{"400 matches nothing", 400, wkerrors.ErrAPINotFound, false},
		{"404 does not match unrelated", 404, errors.New("other"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := &APIError{StatusCode: tc.status, Message: "x"}
			got := errors.Is(err, tc.target)
			if got != tc.want {
				t.Errorf("errors.Is(APIError{%d}, %v) = %v, want %v", tc.status, tc.target, got, tc.want)
			}
		})
	}
}

// Regression test for ADR-005 Decision 9: a wrapped APIError must still
// satisfy errors.Is against the sentinel so the sync engine's 404 retry
// path can detect a stale folder_id cache.
func TestAPIError_IsThroughWrap(t *testing.T) {
	err := &APIError{StatusCode: 404, Message: "Not found"}
	wrapped := fmt.Errorf("creating export manifest: %w", err)
	if !errors.Is(wrapped, wkerrors.ErrAPINotFound) {
		t.Fatalf("wrapped APIError{404} did not match ErrAPINotFound; errors.Is returned false")
	}
}
