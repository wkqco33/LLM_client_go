package gemini

import llm "llm-client-go"

func NewUserMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleUser, Content: content}
}

func NewSystemMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleSystem, Content: content}
}

func NewAssistantMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleAssistant, Content: content}
}

// NewToolResultMessage creates a tool result message for Gemini.
// functionName must be the name of the function that was called,
// since Gemini uses function names (not IDs) to match tool results.
func NewToolResultMessage(functionName, content string) llm.Message {
	return llm.Message{Role: llm.RoleTool, ToolCallID: functionName, Content: content}
}

func NewTool(name, description string, parameters any) llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}
