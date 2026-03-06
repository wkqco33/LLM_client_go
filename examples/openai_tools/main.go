package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	llm "llm-client-go"
	"llm-client-go/examples/internal/dotenv"
	"llm-client-go/openai"
)

// getWeather is a mock tool implementation.
func getWeather(city string) string {
	return fmt.Sprintf("The weather in %s is sunny and 22°C.", city)
}

func main() {
	if err := dotenv.Load(); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	client := openai.New(openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	weatherTool := openai.NewTool("get_weather", "Get the current weather for a city", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "The name of the city",
			},
		},
		"required": []string{"city"},
	})

	messages := []llm.Message{
		openai.NewUserMessage("What's the weather like in Tokyo?"),
	}

	// First turn: model may request a tool call
	resp, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: messages,
		Tools:    []llm.Tool{weatherTool},
	})
	if err != nil {
		log.Fatal(err)
	}

	assistantMsg := resp.Choices[0].Message
	fmt.Printf("Finish reason: %s\n", resp.Choices[0].FinishReason)

	if resp.Choices[0].FinishReason != "tool_calls" {
		// No tool call requested; print and exit
		fmt.Println(assistantMsg.Content)
		return
	}

	// Append the assistant message (with ToolCalls) to the conversation
	messages = append(messages, assistantMsg)

	// Execute each tool call and append the results
	for _, tc := range assistantMsg.ToolCalls {
		fmt.Printf("Calling tool: %s(%s)\n", tc.Function.Name, tc.Function.Arguments)

		var args map[string]string
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			log.Fatal(err)
		}

		result := getWeather(args["city"])
		fmt.Printf("Tool result: %s\n", result)

		messages = append(messages, openai.NewToolResultMessage(tc.ID, result))
	}

	// Second turn: send tool results back to the model
	finalResp, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: messages,
		Tools:    []llm.Tool{weatherTool},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nAssistant: %s\n", finalResp.Choices[0].Message.Content)
}
