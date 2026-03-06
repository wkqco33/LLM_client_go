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

	var client llm.Client = azure.New(azure.Config{
		Endpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
		APIKey:   os.Getenv("AZURE_OPENAI_API_KEY"),
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: os.Getenv("AZURE_OPENAI_DEPLOYMENT"),
		Messages: []llm.Message{
			azure.NewSystemMessage("You are a helpful assistant."),
			azure.NewUserMessage("Write a short poem about Azure OpenAI."),
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

		if len(chunk.Choices) > 0 {
			fmt.Print(chunk.Choices[0].Delta.Content)
		}
	}
	fmt.Println()
}
