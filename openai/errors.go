package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/internal/apierr"
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
		return apierr.Wrap(resp.StatusCode, apiErr)
	}

	// Fallback: return a plain API error with the raw body.
	apiErr := &llm.APIError{
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}
	return apierr.Wrap(resp.StatusCode, apiErr)
}

type openAIErrorBody struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
}
