package gemini

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

	var gErr struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	json.Unmarshal(body, &gErr)

	apiErr := &llm.APIError{
		StatusCode: resp.StatusCode,
		Type:       gErr.Error.Status,
		Message:    gErr.Error.Message,
	}
	if apiErr.Message == "" {
		apiErr.Message = string(body)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
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
