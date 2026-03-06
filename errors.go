package llm

import (
	"errors"
	"fmt"
)

// APIError represents an error returned by an LLM provider API.
type APIError struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Type       string `json:"type"`
	Param      string `json:"param,omitempty"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("llm api error (status=%d, code=%s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("llm api error (status=%d): %s", e.StatusCode, e.Message)
}

// IsAPIError reports whether err is (or wraps) an *APIError and writes it to target.
func IsAPIError(err error, target **APIError) bool {
	return errors.As(err, target)
}

// Sentinel errors for common HTTP status conditions.
var (
	// ErrUnauthorized is returned when the API key is invalid or missing (HTTP 401).
	ErrUnauthorized = errors.New("unauthorized: invalid or missing API key")

	// ErrRateLimited is returned when the rate limit is exceeded (HTTP 429).
	ErrRateLimited = errors.New("rate limited: too many requests")

	// ErrNotFound is returned when the requested resource does not exist (HTTP 404).
	ErrNotFound = errors.New("not found")

	// ErrBadRequest is returned for malformed requests (HTTP 400).
	ErrBadRequest = errors.New("bad request")

	// ErrServerError is returned for provider-side failures (HTTP 5xx).
	ErrServerError = errors.New("server error")

	// ErrStreamClosed is returned when reading from an already-closed stream.
	ErrStreamClosed = errors.New("stream is closed")
)
