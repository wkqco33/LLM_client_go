package openai_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/openai"
	"github.com/wkqco33/LLM_client_go/retry"
)

func ExampleNew() {
	client := openai.New(openai.Config{
		APIKey: "dummy-key",
	})
	fmt.Printf("Client initialized: %t\n", client != nil)
	// Output:
	// Client initialized: true
}

func ExampleClient_Complete() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello! How can I help you today?"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 9, "completion_tokens": 12, "total_tokens": 21}
		}`))
	}))
	defer srv.Close()

	client := openai.New(openai.Config{
		APIKey:      "dummy-key",
		BaseURL:     srv.URL,
		RetryPolicy: &retry.Policy{},
	})

	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gpt-4o",
		Messages: []llm.Message{
			openai.NewUserMessage("Hello!"),
		},
	})
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	fmt.Println(resp.Choices[0].Message.Content)
	// Output:
	// Hello! How can I help you today?
}
