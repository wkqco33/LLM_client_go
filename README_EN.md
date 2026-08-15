# LLM Client Go

[![Go Reference](https://pkg.go.dev/badge/github.com/wkqco33/LLM_client_go.svg)](https://pkg.go.dev/github.com/wkqco33/LLM_client_go)
[![CI](https://github.com/wkqco33/LLM_client_go/actions/workflows/ci.yml/badge.svg)](https://github.com/wkqco33/LLM_client_go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/wkqco33/LLM_client_go)](https://goreportcard.com/report/github.com/wkqco33/LLM_client_go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A unified LLM client and agent framework implemented in Go.  
Seamlessly control **OpenAI**, **Azure OpenAI**, and **Ollama (local)** through a single interface, featuring **Model Context Protocol (MCP)** integration, automated tool-execution agents, and built-in retries.

[한국어 README](README.md)

---

## Installation

```bash
go get github.com/wkqco33/LLM_client_go
```

## Key Features

| Feature | Description |
| - | - |
| **Unified Interface** | Control OpenAI, Azure OpenAI, and Ollama models with a single `llm.Client` interface |
| **Automated Agent** | Multi-turn agent loop with parallel tool calling via `agent.Runner` |
| **MCP Integration** | Connect tools from external MCP servers via HTTP and Stdio (JSON-RPC) |
| **Built-in Resilience** | Automatic exponential backoff retry middleware with jitter |
| **RAG Support** | Unified interface for Embeddings generation |
| **Utilities** | Heuristic token counter and conversation context management |
| **Bot Adapters** | Ready-to-use bot integrations for Discord, Telegram, and Slack |

---

## Quickstart

### 1. Unified Client

```go
package main

import (
    "context"
    "fmt"
    "os"

    llm "github.com/wkqco33/LLM_client_go"
    "github.com/wkqco33/LLM_client_go/openai"
)

func main() {
    client := openai.New(openai.Config{
        APIKey: os.Getenv("OPENAI_API_KEY"),
    })

    resp, err := client.Complete(context.Background(), llm.ChatRequest{
        Model: "gpt-4o",
        Messages: []llm.Message{
            {Role: llm.RoleUser, Content: "Hello, what can you do?"},
        },
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

### 2. Autonomous Agent with MCP Tools

```go
package main

import (
    "context"
    "fmt"
    "os"

    llm "github.com/wkqco33/LLM_client_go"
    "github.com/wkqco33/LLM_client_go/agent"
    "github.com/wkqco33/LLM_client_go/mcp"
    "github.com/wkqco33/LLM_client_go/openai"
)

func main() {
    ctx := context.Background()
    client := openai.New(openai.Config{APIKey: os.Getenv("OPENAI_API_KEY")})

    // 1. Connect to an external MCP server (stdio transport)
    mcpClient, err := mcp.NewStdioClient("npx", "-y", "@modelcontextprotocol/server-filesystem", "./")
    if err != nil {
        panic(err)
    }
    defer mcpClient.Close()

    // 2. Initialize Agent Runner
    runner := agent.NewRunner(client, "gpt-4o",
        agent.WithSystemPrompt("You are an assistant capable of reading and managing files."),
    )

    // 3. Register MCP tools to the agent
    tools, _ := mcpClient.ListTools(ctx)
    for _, tool := range mcp.WrapTools(mcpClient, tools) {
        runner.RegisterTool(tool)
    }

    // 4. Run the multi-turn agent loop
    _, resp, err := runner.Run(ctx, []llm.Message{
        {Role: llm.RoleUser, Content: "List all files in the current directory"},
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

---

## Testing & Development

This project follows strict Test-Driven Development (TDD).

```bash
# Run all tests
task test

# Run tests in verbose mode
task test:verbose

# Run test coverage
task test:coverage

# Watch mode for TDD loop
task --watch test
```

---

## License

This project is licensed under the [MIT License](LICENSE).
