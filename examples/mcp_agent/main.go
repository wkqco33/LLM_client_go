package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/agent"
	"github.com/wkqco33/LLM_client_go/examples/internal/dotenv"
	"github.com/wkqco33/LLM_client_go/mcp"
	"github.com/wkqco33/LLM_client_go/openai"
)

// ─── Mock MCP Server ──────────────────────────────────────────

func startMockMCPServer() *httptest.Server {
	mux := http.NewServeMux()

	// 1. List tools
	mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tools": []mcp.Tool{
				{
					Name:        "get_weather",
					Description: "Get the current weather for a city",
					InputSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"city": map[string]any{"type": "string"},
						},
						"required": []string{"city"},
					},
				},
			},
		})
	})

	// 2. Call tool
	mux.HandleFunc("/tools/call", func(w http.ResponseWriter, r *http.Request) {
		var req mcp.CallToolRequest
		json.NewDecoder(r.Body).Decode(&req)

		fmt.Printf(">> [MCP Server] Executing tool: %s with args: %+v\n", req.Name, req.Arguments)

		city := req.Arguments["city"].(string)
		result := mcp.CallToolResponse{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: fmt.Sprintf("The weather in %s is cloudy but warm.", city)},
			},
		}
		json.NewEncoder(w).Encode(result)
	})

	return httptest.NewServer(mux)
}

// ─── Main Program ──────────────────────────────────────────────

func main() {
	if err := dotenv.Load(); err != nil {
		log.Printf("Warning: failed to load .env: %v", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is not set")
	}

	// 1. Start a mock MCP server locally
	mcpServer := startMockMCPServer()
	defer mcpServer.Close()
	fmt.Printf("MCP Server started at: %s\n", mcpServer.URL)

	// 2. Setup LLM Client & Agent Runner
	client := openai.New(openai.Config{APIKey: apiKey})
	runner := agent.NewRunner(client, "gpt-4o", agent.WithSystemPrompt("You are a helpful assistant with access to MCP tools."))

	// 3. Connect to MCP server and fetch tools
	mcpClient := mcp.NewHttpClient(mcpServer.URL)
	ctx := context.Background()

	mcpTools, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Fatalf("failed to list MCP tools: %v", err)
	}

	// 4. Wrap MCP tools and register them to the Agent
	executableTools := mcp.WrapTools(mcpClient, mcpTools)
	for _, t := range executableTools {
		fmt.Printf("Registering MCP tool: %s\n", t.Definition().Function.Name)
		runner.RegisterTool(t)
	}

	// 5. Run conversation
	fmt.Println("\nUser: What is the weather in London?")
	msgs, finalResp, err := runner.Run(ctx, []llm.Message{
		openai.NewUserMessage("What is the weather in London?"),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nAssistant: %s\n", finalResp.Choices[0].Message.Content)

	// Check if the conversation actually used the MCP tool
	fmt.Println("\n--- Execution Trace ---")
	for _, m := range msgs {
		if m.Role == llm.RoleAssistant && len(m.ToolCalls) > 0 {
			fmt.Printf("[Assistant] Asked to call MCP tool: %s\n", m.ToolCalls[0].Function.Name)
		} else if m.Role == llm.RoleTool {
			fmt.Printf("[Tool Result] %s\n", m.Content)
		}
	}
}
