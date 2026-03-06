package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	llm "llm-client-go"
)

// parseErrorResponse reads an error response body and returns a structured error.
func parseErrorResponse(resp *http.Response) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// OpenAI wraps the error under an "error" key.
	var wrapper struct {
		Error *openAIErrorBody `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Error != nil {
		apiErr := &llm.APIError{
			StatusCode: resp.StatusCode,
			Code:       fmt.Sprintf("%v", wrapper.Error.Code),
			Message:    wrapper.Error.Message,
			Type:       wrapper.Error.Type,
			Param:      wrapper.Error.Param,
		}
		return wrapStatusError(resp.StatusCode, apiErr)
	}

	// Fallback: return a plain API error with the raw body.
	apiErr := &llm.APIError{
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}
	return wrapStatusError(resp.StatusCode, apiErr)
}

type openAIErrorBody struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
}

func wrapStatusError(statusCode int, apiErr *llm.APIError) error {
	switch statusCode {
	case http.StatusUnauthorized:
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
