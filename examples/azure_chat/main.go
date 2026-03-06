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

	client := azure.New(azure.Config{
		Endpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"), // e.g. https://my-resource.openai.azure.com
		APIKey:   os.Getenv("AZURE_OPENAI_API_KEY"),
	})

	resp, err := client.Chat.Complete(context.Background(), azure.ChatRequest{
		DeploymentName: os.Getenv("AZURE_OPENAI_DEPLOYMENT"), // e.g. "gpt-4o"
		Messages: []llm.Message{
			azure.NewSystemMessage("You are a helpful assistant."),
			azure.NewUserMessage("What is the capital of Japan?"),
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
