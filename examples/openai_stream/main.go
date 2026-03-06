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

	var client llm.Client = openai.New(openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gpt-4o",
		Messages: []llm.Message{
			openai.NewSystemMessage("You are a helpful assistant."),
			openai.NewUserMessage("Write a short poem about Go programming."),
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
