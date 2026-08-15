package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/agent"
)

// mockClient simulates an LLM. It responds with tool calls or a final answer based on the input.
type mockClient struct {
	turnCount int
}

func (m *mockClient) Complete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.turnCount++

	// First turn: model asks to call "get_weather"
	if m.turnCount == 1 {
		return &llm.ChatResponse{
			Choices: []llm.Choice{{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:   "call_1",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"city":"Seoul"}`,
						},
					}},
				},
				FinishReason: "tool_calls",
			}},
		}, nil
	}

	// Second turn: model receives the tool result and generates final answer
	lastMsg := req.Messages[len(req.Messages)-1]
	if lastMsg.Role != llm.RoleTool {
		return nil, fmt.Errorf("expected last message to be from tool, got %q", lastMsg.Role)
	}

	return &llm.ChatResponse{
		Choices: []llm.Choice{{
			Message:      llm.Message{Role: llm.RoleAssistant, Content: "The weather in Seoul is " + lastMsg.Content},
			FinishReason: "stop",
		}},
	}, nil
}

func (m *mockClient) Stream(ctx context.Context, req llm.ChatRequest) (llm.Stream, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClient) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClient) TokenCounter(model string) any {
	return nil
}

// weatherTool is a mock executable tool.
type weatherTool struct{}

func (w *weatherTool) Definition() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "get_weather",
			Description: "Get weather for a city",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
			},
		},
	}
}

func (w *weatherTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args map[string]string
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", err
	}
	if args["city"] == "Seoul" {
		return "Sunny and 20 degrees", nil
	}
	return "Unknown", nil
}

func TestRunner_Run(t *testing.T) {
	client := &mockClient{}
	runner := agent.NewRunner(client, "test-model", agent.WithSystemPrompt("You are an assistant."))
	runner.RegisterTool(&weatherTool{})

	msgs, resp, err := runner.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "What is the weather in Seoul?"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.turnCount != 2 {
		t.Errorf("expected exactly 2 turns, got %d", client.turnCount)
	}

	finalReply := resp.Choices[0].Message.Content
	if !strings.Contains(finalReply, "Sunny and 20 degrees") {
		t.Errorf("unexpected final reply: %s", finalReply)
	}

	// Verify history contains System -> User -> Assistant(ToolCall) -> Tool -> Assistant(Final)
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages in history, got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("first message should be system, got %v", msgs[0].Role)
	}
	if msgs[3].Role != llm.RoleTool {
		t.Errorf("fourth message should be tool result, got %v", msgs[3].Role)
	}
}
