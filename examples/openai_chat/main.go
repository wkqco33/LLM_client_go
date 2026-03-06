package main

import (
	"context"
	"fmt"
	"log"
	"os"

	llm "llm-client-go"
	"llm-client-go/examples/internal/dotenv"
	"llm-client-go/openai"
)

func main() {
	if err := dotenv.Load(); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	client := openai.New(openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	resp, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-4o",
		Messages: []llm.Message{
			openai.NewSystemMessage("You are a helpful assistant."),
			openai.NewUserMessage("What is the capital of France?"),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Choices[0].Message.Content)
	fmt.Printf("\nUsage — prompt: %d, completion: %d, total: %d\n",
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens,
	)
}
