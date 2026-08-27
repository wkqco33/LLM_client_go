package llm_test

import (
	"fmt"

	llm "github.com/wkqco33/LLM_client_go"
)

func ExampleNewUserMessage() {
	msg := llm.NewUserMessage("Hello, world!")
	fmt.Println(msg.Role, msg.Content)
	// Output:
	// user Hello, world!
}

func ExampleNewTool() {
	tool := llm.NewTool("get_weather", "Get current weather in a city", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string"},
		},
		"required": []string{"city"},
	})
	fmt.Println(tool.Function.Name, tool.Function.Description)
	// Output:
	// get_weather Get current weather in a city
}

func ExampleCollectToolCalls() {
	deltas := []llm.ToolCallDelta{
		{Index: 0, ID: "call_1", Function: llm.FunctionCallDelta{Name: "get_weather", Arguments: `{"city":`}},
		{Index: 0, Function: llm.FunctionCallDelta{Arguments: `"Seoul"}`}},
	}
	calls := llm.CollectToolCalls(deltas)
	fmt.Printf("%s: %s\n", calls[0].Function.Name, calls[0].Function.Arguments)
	// Output:
	// get_weather: {"city":"Seoul"}
}
