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

	stream, err := client.Chat.Stream(context.Background(), openai.ChatRequest{
		Model: "gpt-4o",
		Messages: []llm.Message{
			openai.NewUserMessage("Tell me a short story about a robot."),
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
			// [DONE]
			break
		}
		for _, choice := range chunk.Choices {
			fmt.Print(choice.Delta.Content)
		}
	}
	fmt.Println()
}
