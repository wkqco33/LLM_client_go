package bots

import (
	"context"
	"fmt"

	llm "llm-client-go"
	"llm-client-go/azure"
	"llm-client-go/openai"
)

// Backend is the interface any LLM provider must satisfy to be used by a bot.
type Backend interface {
	// Complete sends the full conversation history to the LLM and returns the assistant reply.
	Complete(ctx context.Context, messages []llm.Message) (string, error)
}

// CommonBackend wraps any llm.Client as a Bot Backend.
type CommonBackend struct {
	Client llm.Client
	Model  string
}

// NewOpenAIBackend creates an OpenAI-backed Backend using the unified interface.
func NewOpenAIBackend(apiKey, model string, opts ...openai.Option) *CommonBackend {
	return &CommonBackend{
		Client: openai.New(openai.Config{APIKey: apiKey}, opts...),
		Model:  model,
	}
}

// NewAzureBackend creates an Azure OpenAI-backed Backend using the unified interface.
func NewAzureBackend(endpoint, apiKey, deploymentName string, opts ...azure.Option) *CommonBackend {
	return &CommonBackend{
		Client: azure.New(azure.Config{Endpoint: endpoint, APIKey: apiKey}, opts...),
		Model:  deploymentName,
	}
}

// Complete implements Backend using the unified llm.Client interface.
func (b *CommonBackend) Complete(ctx context.Context, messages []llm.Message) (string, error) {
	resp, err := b.Client.Complete(ctx, llm.ChatRequest{
		Model:    b.Model,
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("backend: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("backend: no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
