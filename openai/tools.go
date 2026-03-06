package openai

import (
	llm "llm-client-go"
)

// NewTool is a convenience constructor for a function-type Tool.
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

// ForceToolChoice returns a ToolChoice that forces the model to call the named function.
func ForceToolChoice(functionName string) llm.SpecificToolChoice {
	return llm.SpecificToolChoice{
		Type:     "function",
		Function: llm.SpecificToolFunction{Name: functionName},
	}
}

// CollectToolCalls assembles all streamed ToolCallDelta fragments into a slice of llm.ToolCall.
// Pass the accumulated deltas from all chunks for a single message.
func CollectToolCalls(deltas []llm.ToolCallDelta) []llm.ToolCall {
	// Index → assembled ToolCall
	indexed := make(map[int]*llm.ToolCall)
	order := []int{}

	for _, d := range deltas {
		tc, ok := indexed[d.Index]
		if !ok {
			tc = &llm.ToolCall{Type: "function"}
			indexed[d.Index] = tc
			order = append(order, d.Index)
		}
		if d.ID != "" {
			tc.ID = d.ID
		}
		if d.Function.Name != "" {
			tc.Function.Name += d.Function.Name
		}
		if d.Function.Arguments != "" {
			tc.Function.Arguments += d.Function.Arguments
		}
	}

	result := make([]llm.ToolCall, 0, len(order))
	for _, idx := range order {
		result = append(result, *indexed[idx])
	}
	return result
}
