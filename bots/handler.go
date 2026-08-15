package bots

import (
	"context"
	"fmt"
	"strings"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/azure"
	"github.com/wkqco33/LLM_client_go/ollama"
	"github.com/wkqco33/LLM_client_go/openai"
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

// NewOllamaBackend creates an Ollama-backed Backend using the unified
// interface. Ollama runs locally and requires no API key; baseURL may be
// "" to use the default (http://localhost:11434/v1).
func NewOllamaBackend(baseURL, model string, opts ...openai.Option) *CommonBackend {
	return &CommonBackend{
		Client: ollama.New(ollama.Config{BaseURL: baseURL}, opts...),
		Model:  model,
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

// HandleTurn processes one incoming user message against sessions/backend:
// it detects the reset command, otherwise appends the user message, calls
// the backend with the full history, and appends the assistant reply.
//
// Every platform adapter (Discord, Telegram, Slack) drove this same
// sequence independently; it's centralized here so the behavior (including
// which errors are returned to the caller to render) stays one
// implementation instead of three near-identical copies.
//
// If the message is a reset command, wasReset is true and reply is the
// confirmation text to send; err is always nil in that case. Otherwise,
// reply is the assistant's response, or "" with a non-nil err if the
// backend call failed (the caller decides how to render that failure).
func HandleTurn(ctx context.Context, sessions *SessionManager, backend Backend, userID, text, resetCmd string) (reply string, wasReset bool, err error) {
	if strings.EqualFold(text, resetCmd) {
		sessions.Reset(userID)
		return "✅ Conversation reset.", true, nil
	}

	sessions.Append(userID, llm.Message{Role: llm.RoleUser, Content: text})
	history := sessions.GetHistory(userID)

	reply, err = backend.Complete(ctx, history)
	if err != nil {
		return "", false, err
	}

	sessions.Append(userID, llm.Message{Role: llm.RoleAssistant, Content: reply})
	return reply, false, nil
}
