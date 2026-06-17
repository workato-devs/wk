package api

import (
	"fmt"

	wkerrors "github.com/workato-devs/wk/internal/errors"
)

// APIError represents an error response from the Workato API.
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	ErrorType  string `json:"error_type,omitempty"`
}

func (e *APIError) Error() string {
	if e.ErrorType != "" {
		return fmt.Sprintf("API error %d (%s): %s", e.StatusCode, e.ErrorType, e.Message)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// Is maps HTTP status codes to the sentinel errors in internal/errors so
// callers can use errors.Is(err, wkerrors.ErrAPINotFound) without reaching
// into the concrete type. 5xx responses all match ErrAPIServer.
func (e *APIError) Is(target error) bool {
	switch target {
	case wkerrors.ErrAPIUnauthorized:
		return e.StatusCode == 401
	case wkerrors.ErrAPIForbidden:
		return e.StatusCode == 403
	case wkerrors.ErrAPINotFound:
		return e.StatusCode == 404
	case wkerrors.ErrAPIRateLimit:
		return e.StatusCode == 429
	case wkerrors.ErrAPIServer:
		return e.StatusCode >= 500 && e.StatusCode < 600
	}
	return false
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsUnauthorized returns true if the error is a 401.
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == 401
}

// IsRateLimit returns true if the error is a 429.
func (e *APIError) IsRateLimit() bool {
	return e.StatusCode == 429
}
