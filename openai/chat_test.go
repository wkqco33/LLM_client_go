package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	llm "llm-client-go"
	"llm-client-go/openai"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*openai.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := openai.New(openai.Config{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	return client, srv
}

func TestChatComplete_Success(t *testing.T) {
	fixture := openai.ChatResponse{
		ID:    "chatcmpl-test",
		Model: "gpt-4o",
		Choices: []openai.Choice{
			{
				Index:        0,
				Message:      llm.Message{Role: llm.RoleAssistant, Content: "Paris"},
				FinishReason: "stop",
			},
		},
		Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or invalid Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fixture)
	})

	resp, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("Capital of France?")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].Message.Content != "Paris" {
		t.Errorf("expected 'Paris', got %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected TotalTokens=15, got %d", resp.Usage.TotalTokens)
	}
}

func TestChatComplete_APIError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Incorrect API key provided",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
	})

	_, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("Hello")},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *llm.APIError
	if !llm.IsAPIError(err, &apiErr) {
		t.Fatalf("expected *llm.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", apiErr.StatusCode)
	}
}

func TestChatComplete_WithTools(t *testing.T) {
	fixture := openai.ChatResponse{
		ID:    "chatcmpl-tools",
		Model: "gpt-4o",
		Choices: []openai.Choice{
			{
				Index: 0,
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call_abc123",
							Type: "function",
							Function: llm.FunctionCall{
								Name:      "get_weather",
								Arguments: `{"city":"Tokyo"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify the request includes tools
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["tools"]; !ok {
			t.Error("expected 'tools' field in request body")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fixture)
	})

	weatherTool := openai.NewTool("get_weather", "Get weather", map[string]any{
		"type":       "object",
		"properties": map[string]any{"city": map[string]any{"type": "string"}},
	})

	resp, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("Weather in Tokyo?")},
		Tools:    []llm.Tool{weatherTool},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("expected finish_reason='tool_calls', got %q", resp.Choices[0].FinishReason)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
}
