package anthropic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	llm "llm-client-go"
)

func parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var antErr struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(body, &antErr)

	apiErr := &llm.APIError{
		StatusCode: resp.StatusCode,
		Type:       antErr.Error.Type,
		Message:    antErr.Error.Message,
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", llm.ErrUnauthorized, apiErr.Error())
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", llm.ErrRateLimited, apiErr.Error())
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", llm.ErrNotFound, apiErr.Error())
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", llm.ErrBadRequest, apiErr.Error())
	default:
		return fmt.Errorf("%w: %s", llm.ErrServerError, apiErr.Error())
	}
}
