package main

import (
	"context"
	"fmt"
	"log"
	"os"

	llm "llm-client-go"
	"llm-client-go/azure"
	"llm-client-go/examples/internal/dotenv"
)

func main() {
	if err := dotenv.Load(); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	// azure.Client implements llm.Client interface
	var client llm.Client = azure.New(azure.Config{
		Endpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
		APIKey:   os.Getenv("AZURE_OPENAI_API_KEY"),
	})

	// DeploymentName is mapped to Model in llm.ChatRequest
	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: os.Getenv("AZURE_OPENAI_DEPLOYMENT"),
		Messages: []llm.Message{
			azure.NewSystemMessage("You are a helpful assistant."),
			azure.NewUserMessage("What is the capital of France?"),
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
