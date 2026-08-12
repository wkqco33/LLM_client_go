package azure

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	llm "llm-client-go"
	"llm-client-go/internal/apierr"
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
		return apierr.Wrap(resp.StatusCode, apiErr)
	}

	apiErr := &llm.APIError{
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}
	return apierr.Wrap(resp.StatusCode, apiErr)
}

type azureErrorBody struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
}
