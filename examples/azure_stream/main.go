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
		Endpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
		APIKey:   os.Getenv("AZURE_OPENAI_API_KEY"),
	})

	stream, err := client.Chat.Stream(context.Background(), azure.ChatRequest{
		DeploymentName: os.Getenv("AZURE_OPENAI_DEPLOYMENT"),
		Messages: []llm.Message{
			azure.NewUserMessage("Tell me a short story about the ocean."),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	fmt.Print("Assistant: ")
	for {
		chunk, err := stream.Next()
		if err != nil {
			log.Fatal(err)
		}
		if chunk == nil {
			break
		}
		for _, choice := range chunk.Choices {
			fmt.Print(choice.Delta.Content)
		}
	}
	fmt.Println()
}
