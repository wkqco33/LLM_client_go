package azure

import (
	llm "github.com/wkqco33/LLM_client_go"
)

// Re-export common message/tool constructors from the root llm package.
var (
	NewUserMessage       = llm.NewUserMessage
	NewSystemMessage     = llm.NewSystemMessage
	NewAssistantMessage  = llm.NewAssistantMessage
	NewToolResultMessage = llm.NewToolResultMessage
	NewTool              = llm.NewTool
	ForceToolChoice      = llm.ForceToolChoice
	CollectToolCalls     = llm.CollectToolCalls
)
