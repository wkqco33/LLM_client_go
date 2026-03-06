package azure

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	llm "llm-client-go"
)

func parseErrorResponse(resp *http.Response) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Azure wraps errors under an "error" key, same structure as OpenAI.
	var wrapper struct {
		Error *azureErrorBody `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Error != nil {
		apiErr := &llm.APIError{
			StatusCode: resp.StatusCode,
			Code:       fmt.Sprintf("%v", wrapper.Error.Code),
			Message:    wrapper.Error.Message,
		}
		return wrapStatusError(resp.StatusCode, apiErr)
	}

	apiErr := &llm.APIError{
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}
	return wrapStatusError(resp.StatusCode, apiErr)
}

type azureErrorBody struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
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
