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

// OpenAIBackend wraps the OpenAI client as a Bot Backend.
type OpenAIBackend struct {
	Client *openai.Client
	Model  string
}

// NewOpenAIBackend creates an OpenAI-backed Backend.
func NewOpenAIBackend(apiKey, model string, opts ...openai.Option) *OpenAIBackend {
	return &OpenAIBackend{
		Client: openai.New(openai.Config{APIKey: apiKey}, opts...),
		Model:  model,
	}
}

// Complete implements Backend for OpenAI.
func (b *OpenAIBackend) Complete(ctx context.Context, messages []llm.Message) (string, error) {
	resp, err := b.Client.Chat.Complete(ctx, openai.ChatRequest{
		Model:    b.Model,
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("openai backend: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai backend: no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}

// AzureBackend wraps the Azure OpenAI client as a Bot Backend.
type AzureBackend struct {
	Client         *azure.Client
	DeploymentName string
}

// NewAzureBackend creates an Azure OpenAI-backed Backend.
func NewAzureBackend(endpoint, apiKey, deploymentName string, opts ...azure.Option) *AzureBackend {
	return &AzureBackend{
		Client:         azure.New(azure.Config{Endpoint: endpoint, APIKey: apiKey}, opts...),
		DeploymentName: deploymentName,
	}
}

// Complete implements Backend for Azure OpenAI.
func (b *AzureBackend) Complete(ctx context.Context, messages []llm.Message) (string, error) {
	resp, err := b.Client.Chat.Complete(ctx, azure.ChatRequest{
		DeploymentName: b.DeploymentName,
		Messages:       messages,
	})
	if err != nil {
		return "", fmt.Errorf("azure backend: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("azure backend: no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
