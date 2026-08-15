// Command ollama_chat demonstrates talking to a local Ollama server.
// Ollama exposes an OpenAI-compatible API, so ollama.New returns a regular
// *openai.Client — no separate provider API to learn, and no API key needed.
//
// Start Ollama and pull a model first:
//
//	ollama pull llama3.2
//	ollama serve
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/examples/internal/dotenv"
	"github.com/wkqco33/LLM_client_go/ollama"
	"github.com/wkqco33/LLM_client_go/openai"
)

func main() {
	if err := dotenv.Load(); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama3.2"
	}

	// ollama.New returns *openai.Client, which implements llm.Client.
	var client llm.Client = ollama.New(ollama.Config{
		BaseURL: os.Getenv("OLLAMA_BASE_URL"), // defaults to http://localhost:11434/v1
	})

	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: model,
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
