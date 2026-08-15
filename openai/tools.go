package openai

import (
	llm "github.com/wkqco33/LLM_client_go"
)

// Message/tool constructors and stream-delta assembly re-exported from the
// root llm package, which is where the (provider-independent)
// implementation lives.
//
// Example:
//
//	tool := openai.NewTool("get_weather", "Get the weather for a city", map[string]any{
//	    "type": "object",
//	    "properties": map[string]any{
//	        "city": map[string]any{"type": "string"},
//	    },
//	    "required": []string{"city"},
//	})
var (
	NewUserMessage       = llm.NewUserMessage
	NewSystemMessage     = llm.NewSystemMessage
	NewAssistantMessage  = llm.NewAssistantMessage
	NewToolResultMessage = llm.NewToolResultMessage
	NewTool              = llm.NewTool
	ForceToolChoice      = llm.ForceToolChoice
	CollectToolCalls     = llm.CollectToolCalls
)
