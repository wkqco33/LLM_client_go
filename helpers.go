package llm

// Common constructors and helpers for messages, tools, and stream assembly.

import "encoding/base64"

// NewUserMessage creates a user-role message.
func NewUserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content}
}

// TextContent creates a text part for a multimodal message.
func TextContent(text string) ContentPart {
	return ContentPart{Type: "text", Text: text}
}

// ImageContent creates an image part from an https or data URL.
func ImageContent(url string) ContentPart {
	return ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: url},
	}
}

// ImageContentData creates an image part from raw image bytes. mediaType
// should be an image MIME type such as image/png or image/jpeg.
func ImageContentData(data []byte, mediaType string) ContentPart {
	return ImageContent("data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data))
}

// NewUserMessageWithParts creates a user message containing text and/or images.
func NewUserMessageWithParts(parts ...ContentPart) Message {
	return Message{Role: RoleUser, ContentParts: parts}
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
	if len(deltas) == 0 {
		return nil
	}

	indexed := make(map[int]*ToolCall, len(deltas))
	order := make([]int, 0, len(deltas))

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
