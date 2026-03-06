package openai_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	llm "llm-client-go"
	"llm-client-go/openai"
)

func TestChatStream_Success(t *testing.T) {
	// Simulate an SSE stream with 3 content chunks + [DONE]
	ssePayload := "" +
		"data: {\"id\":\"s1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"s1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\", world\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"s1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprint(w, ssePayload)
	}))
	defer srv.Close()

	client := openai.New(openai.Config{APIKey: "test-key", BaseURL: srv.URL})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("Hi")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stream.Close()

	var collected string
	for {
		chunk, err := stream.Next()
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		if chunk == nil {
			break
		}
		for _, c := range chunk.Choices {
			collected += c.Delta.Content
		}
	}

	if collected != "Hello, world" {
		t.Errorf("expected 'Hello, world', got %q", collected)
	}
}

func TestChatStream_ClosedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	client := openai.New(openai.Config{APIKey: "test-key", BaseURL: srv.URL})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("Hi")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stream.Close()

	_, err = stream.Next()
	if err != llm.ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestCollectToolCalls(t *testing.T) {
	deltas := []llm.ToolCallDelta{
		{Index: 0, ID: "call_1", Type: "function", Function: llm.FunctionCallDelta{Name: "get_w"}},
		{Index: 0, Function: llm.FunctionCallDelta{Name: "eather", Arguments: `{"ci`}},
		{Index: 0, Function: llm.FunctionCallDelta{Arguments: `ty":"Tokyo"}`}},
	}

	calls := openai.CollectToolCalls(deltas)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].ID != "call_1" {
		t.Errorf("expected ID='call_1', got %q", calls[0].ID)
	}
	if calls[0].Function.Name != "get_weather" {
		t.Errorf("expected Name='get_weather', got %q", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments != `{"city":"Tokyo"}` {
		t.Errorf("unexpected arguments: %q", calls[0].Function.Arguments)
	}
}
