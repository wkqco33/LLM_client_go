package agent_test

import (
	"context"
	"fmt"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/agent"
)

type echoTool struct{}

func (e *echoTool) Definition() llm.Tool {
	return llm.NewTool("echo", "Echoes back input", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string"},
		},
		"required": []string{"text"},
	})
}

func (e *echoTool) Execute(ctx context.Context, arguments string) (string, error) {
	return arguments, nil
}

type staticClient struct{}

func (s *staticClient) Complete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Choices: []llm.Choice{
			{
				Message:      llm.Message{Role: llm.RoleAssistant, Content: "All done!"},
				FinishReason: "stop",
			},
		},
	}, nil
}

func (s *staticClient) Stream(ctx context.Context, req llm.ChatRequest) (llm.Stream, error) {
	return nil, nil
}

func (s *staticClient) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, nil
}

func (s *staticClient) TokenCounter(model string) any {
	return nil
}

func ExampleNewRunner() {
	client := &staticClient{}
	runner := agent.NewRunner(client, "gpt-4o",
		agent.WithSystemPrompt("You are a helpful assistant."),
		agent.WithMaxTurns(3),
	)

	runner.RegisterTool(&echoTool{})

	msgs, resp, err := runner.Run(context.Background(), []llm.Message{
		llm.NewUserMessage("Hi"),
	})
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	fmt.Println(resp.Choices[0].Message.Content)
	fmt.Printf("Total messages: %d\n", len(msgs))
	// Output:
	// All done!
	// Total messages: 3
}
