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

// ChatRequest is the input for a chat completion.
type ChatRequest struct {
	// Model is the model ID to use (e.g., "gpt-4o", "gpt-4-turbo").
	Model string `json:"model"`

	// Messages is the conversation history.
	Messages []llm.Message `json:"messages"`

	// MaxTokens limits the number of tokens in the completion.
	// 0 means the API default.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0–2.0). nil uses the API default.
	Temperature *float64 `json:"temperature,omitempty"`

	// TopP is an alternative to Temperature for nucleus sampling.
	TopP *float64 `json:"top_p,omitempty"`

	// N is how many completions to generate. Defaults to 1.
	N int `json:"n,omitempty"`

	// Stop is a list of sequences that will stop generation.
	Stop []string `json:"stop,omitempty"`

	// Tools is the list of tools the model may call.
	Tools []llm.Tool `json:"tools,omitempty"`

	// ToolChoice controls which tool the model should use.
	// Use llm.ToolChoiceAuto, llm.ToolChoiceNone, llm.ToolChoiceRequired,
	// or a llm.SpecificToolChoice value.
	ToolChoice llm.ToolChoice `json:"tool_choice,omitempty"`

	// Stream must not be set manually; use ChatService.Stream() instead.
	Stream bool `json:"stream,omitempty"`

	// StreamOptions enables usage stats in the last stream chunk when Stream is true.
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

// StreamOptions configures stream-specific behavior.
type StreamOptions struct {
	// IncludeUsage adds a final chunk with token usage when true.
	IncludeUsage bool `json:"include_usage"`
}

// ChatResponse is the response from a non-streaming chat completion.
type ChatResponse struct {
	ID                string    `json:"id"`
	Object            string    `json:"object"`
	Created           int64     `json:"created"`
	Model             string    `json:"model"`
	Choices           []Choice  `json:"choices"`
	Usage             llm.Usage `json:"usage"`
	SystemFingerprint string    `json:"system_fingerprint,omitempty"`
}

// Choice is a single completion candidate.
type Choice struct {
	Index        int         `json:"index"`
	Message      llm.Message `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Complete sends a non-streaming chat completion request.
func (s *ChatService) Complete(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
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

	var result ChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
