package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/agent"
	"github.com/wkqco33/LLM_client_go/examples/internal/dotenv"
	"github.com/wkqco33/LLM_client_go/openai"
)

// weatherTool is our local executable tool.
type weatherTool struct{}

func (w *weatherTool) Definition() llm.Tool {
	return openai.NewTool("get_weather", "Get the current weather for a city", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "The name of the city",
			},
		},
		"required": []string{"city"},
	})
}

// Execute is called automatically by the Agent Runner when the model requests it.
func (w *weatherTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args map[string]string
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", err
	}
	city := args["city"]

	fmt.Printf(">> [Local Execution] Fetching weather for %s...\n", city)

	// In a real app, you would call an external weather API here.
	return fmt.Sprintf("The weather in %s is sunny and 22°C.", city), nil
}

func main() {
	if err := dotenv.Load(); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	client := openai.New(openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// Create an Agent Runner
	runner := agent.NewRunner(client, "gpt-4o", agent.WithSystemPrompt("You are a helpful assistant."))
	runner.RegisterTool(&weatherTool{})

	// Start the conversation
	fmt.Println("User: What's the weather like in Tokyo?")
	msgs, finalResp, err := runner.Run(context.Background(), []llm.Message{
		openai.NewUserMessage("What's the weather like in Tokyo?"),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nAssistant: %s\n", finalResp.Choices[0].Message.Content)

	// (Optional) Print the full message history to see the internal tool calls
	fmt.Println("\n--- Full Conversation History ---")
	for _, m := range msgs {
		role := string(m.Role)
		if m.Role == llm.RoleAssistant && len(m.ToolCalls) > 0 {
			fmt.Printf("[%s] (Tool Call: %s)\n", role, m.ToolCalls[0].Function.Name)
		} else if m.Role == llm.RoleTool {
			fmt.Printf("[%s] %s\n", role, m.Content)
		} else {
			fmt.Printf("[%s] %s\n", role, m.Content)
		}
	}
}
