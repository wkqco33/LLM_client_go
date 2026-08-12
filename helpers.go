package llm

// This file collects message/tool constructors and streamed-tool-call
// assembly that have no provider-specific behavior, so every provider
// package can re-export them instead of reimplementing the same logic.

// NewUserMessage creates a user-role message.
func NewUserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content}
}

// NewSystemMessage creates a system-role message.
func NewSystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: content}
}

// NewAssistantMessage creates an assistant-role message.
func NewAssistantMessage(content string) Message {
	return Message{Role: RoleAssistant, Content: content}
}

// NewToolResultMessage creates a tool-role message containing the result of
// a tool call. toolCallID must match the ID the provider used to identify
// the original call.
func NewToolResultMessage(toolCallID, content string) Message {
	return Message{Role: RoleTool, ToolCallID: toolCallID, Content: content}
}

// NewTool is a convenience constructor for a function-type Tool.
func NewTool(name, description string, parameters any) Tool {
	return Tool{
		Type: "function",
		Function: FunctionDef{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

// ForceToolChoice returns a ToolChoice that forces the model to call the
// named function. Only meaningful for providers that read ChatRequest.ToolChoice
// (currently OpenAI and Azure).
func ForceToolChoice(functionName string) SpecificToolChoice {
	return SpecificToolChoice{
		Type:     "function",
		Function: SpecificToolFunction{Name: functionName},
	}
}

// CollectToolCalls assembles all streamed ToolCallDelta fragments into a
// slice of ToolCall. Pass the accumulated deltas from all chunks for a
// single message. Applicable to providers that stream tool-call arguments
// incrementally (currently OpenAI and Azure, which share the same delta
// shape).
func CollectToolCalls(deltas []ToolCallDelta) []ToolCall {
	indexed := make(map[int]*ToolCall)
	order := []int{}

	for _, d := range deltas {
		tc, ok := indexed[d.Index]
		if !ok {
			tc = &ToolCall{Type: "function"}
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

	result := make([]ToolCall, 0, len(order))
	for _, idx := range order {
		result = append(result, *indexed[idx])
	}
	return result
}
