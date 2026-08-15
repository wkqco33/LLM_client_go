package apierr

import (
	"errors"
	"net/http"
	"testing"

	llm "github.com/wkqco33/LLM_client_go"
)

func TestWrap_StatusCodeMapping(t *testing.T) {
	apiErr := &llm.APIError{
		StatusCode: http.StatusUnauthorized,
		Message:    "invalid api key",
		Type:       "invalid_request_error",
	}

	tests := []struct {
		name       string
		statusCode int
		wantTarget error
	}{
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			wantTarget: llm.ErrUnauthorized,
		},
		{
			name:       "403 Forbidden",
			statusCode: http.StatusForbidden,
			wantTarget: llm.ErrUnauthorized,
		},
		{
			name:       "429 Too Many Requests",
			statusCode: http.StatusTooManyRequests,
			wantTarget: llm.ErrRateLimited,
		},
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			wantTarget: llm.ErrNotFound,
		},
		{
			name:       "400 Bad Request",
			statusCode: http.StatusBadRequest,
			wantTarget: llm.ErrBadRequest,
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			wantTarget: llm.ErrServerError,
		},
		{
			name:       "503 Service Unavailable",
			statusCode: http.StatusServiceUnavailable,
			wantTarget: llm.ErrServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrap(tt.statusCode, apiErr)
			if !errors.Is(err, tt.wantTarget) {
				t.Errorf("Wrap(%d) expected errors.Is(err, %v), got %v", tt.statusCode, tt.wantTarget, err)
			}

			var target *llm.APIError
			if !llm.IsAPIError(err, &target) {
				t.Errorf("Wrap(%d) expected llm.IsAPIError to be true", tt.statusCode)
			}
		})
	}
}

func TestWrap_UnmappedStatus_ReturnsAPIErrorDirectly(t *testing.T) {
	apiErr := &llm.APIError{
		StatusCode: http.StatusTeapot,
		Message:    "i'm a teapot",
	}

	err := Wrap(http.StatusTeapot, apiErr)
	var target *llm.APIError
	if !errors.As(err, &target) {
		t.Errorf("expected *llm.APIError, got %v", err)
	}
	if target.StatusCode != http.StatusTeapot {
		t.Errorf("got status %d, want %d", target.StatusCode, http.StatusTeapot)
	}
}
