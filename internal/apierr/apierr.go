// Package apierr maps HTTP status codes to sentinel errors in the root llm package.
package apierr

import (
	"fmt"
	"net/http"

	llm "github.com/wkqco33/LLM_client_go"
)

// Wrap maps statusCode to the corresponding llm sentinel error and wraps apiErr.
func Wrap(statusCode int, apiErr *llm.APIError) error {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: %w", llm.ErrUnauthorized, apiErr)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %w", llm.ErrRateLimited, apiErr)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %w", llm.ErrNotFound, apiErr)
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %w", llm.ErrBadRequest, apiErr)
	default:
		if statusCode >= 500 {
			return fmt.Errorf("%w: %w", llm.ErrServerError, apiErr)
		}
		return apiErr
	}
}
