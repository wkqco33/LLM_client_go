package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	llm "llm-client-go"
)

// ChatService provides access to the Chat Completions endpoint.
type ChatService struct {
	client *Client
}

// Complete sends a non-streaming chat completion request.
func (s *ChatService) Complete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	req.Stream = false

	resp, err := s.client.do(ctx, http.MethodPost, "/chat/completions", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result llm.ChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
