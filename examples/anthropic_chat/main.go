package main

import (
	"context"
	"fmt"
	"log"
	"os"

	llm "llm-client-go"
	"llm-client-go/anthropic"
	"llm-client-go/examples/internal/dotenv"
)

func main() {
	if err := dotenv.Load(); err != nil {
		log.Printf("Warning: failed to load .env: %v", err)
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set")
	}

	// 1. Create Anthropic client (satisfies llm.Client interface)
	var client llm.Client = anthropic.New(anthropic.Config{
		APIKey: apiKey,
	})

	// 2. Prepare conversation
	messages := []llm.Message{
		anthropic.NewSystemMessage("You are a poetic assistant."),
		anthropic.NewUserMessage("Write a single sentence about the moon."),
	}

	// 3. Send request using the unified interface
	fmt.Println("Sending request to Claude...")
	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:     "claude-3-5-sonnet-20240620",
		MaxTokens: 1024,
		Messages:  messages,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// 4. Print results
	fmt.Printf("\nClaude: %s\n", resp.Choices[0].Message.Content)
	fmt.Printf("\nUsage: Prompt=%d, Completion=%d, Total=%d\n",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens,
	)
}
