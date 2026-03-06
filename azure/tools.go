package azure

import (
	llm "llm-client-go"
)

// NewUserMessage creates a user-role message.
func NewUserMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleUser, Content: content}
}

// NewSystemMessage creates a system-role message.
func NewSystemMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleSystem, Content: content}
}

// NewAssistantMessage creates an assistant-role message.
func NewAssistantMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleAssistant, Content: content}
}

// NewToolResultMessage creates a tool-role message containing the result of a tool call.
func NewToolResultMessage(toolCallID, content string) llm.Message {
	return llm.Message{
		Role:       llm.RoleTool,
		Content:    content,
		ToolCallID: toolCallID,
	}
}

// NewTool is a convenience constructor for a function-type Tool.
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

// ForceToolChoice returns a ToolChoice that forces the model to call the named function.
func ForceToolChoice(functionName string) llm.SpecificToolChoice {
	return llm.SpecificToolChoice{
		Type:     "function",
		Function: llm.SpecificToolFunction{Name: functionName},
	}
}
