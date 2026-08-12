package azure

import (
	llm "llm-client-go"
)

// Message/tool constructors and stream-delta assembly re-exported from the
// root llm package, which is where the (provider-independent)
// implementation lives. CollectToolCalls applies here because Azure OpenAI
// streams tool-call arguments in the same delta shape as OpenAI.
var (
	NewUserMessage       = llm.NewUserMessage
	NewSystemMessage     = llm.NewSystemMessage
	NewAssistantMessage  = llm.NewAssistantMessage
	NewToolResultMessage = llm.NewToolResultMessage
	NewTool              = llm.NewTool
	ForceToolChoice      = llm.ForceToolChoice
	CollectToolCalls     = llm.CollectToolCalls
)
