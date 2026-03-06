package azure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	llm "llm-client-go"
)

// ChatService provides access to the Azure OpenAI Chat Completions endpoint.
type ChatService struct {
	client *Client
}

// ChatRequest is the input for an Azure OpenAI chat completion.
// The model is identified by DeploymentName (the deployment you created in Azure Portal).
type ChatRequest struct {
	// DeploymentName is the name of your Azure OpenAI deployment (required).
	DeploymentName string `json:"-"`

	// Messages is the conversation history.
	Messages []llm.Message `json:"messages"`

	// MaxTokens limits the number of tokens in the completion.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0–2.0).
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
	ToolChoice llm.ToolChoice `json:"tool_choice,omitempty"`

	// Stream must not be set manually; use ChatService.Stream() instead.
	Stream bool `json:"stream,omitempty"`
}

// ChatResponse is the response from a non-streaming Azure chat completion.
type ChatResponse struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choice  `json:"choices"`
	Usage   llm.Usage `json:"usage"`
}

// Choice is a single completion candidate.
type Choice struct {
	Index        int         `json:"index"`
	Message      llm.Message `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Complete sends a non-streaming chat completion request to Azure OpenAI.
func (s *ChatService) Complete(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	url := s.client.deploymentURL(req.DeploymentName, "/chat/completions")

	resp, err := s.client.do(ctx, http.MethodPost, url, req)
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
