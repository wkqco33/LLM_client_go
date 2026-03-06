// Package llm provides common types shared across LLM provider implementations.
package llm

// Role represents the role of a message participant.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message represents a single chat message.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// Tool represents a tool (function) that the model can call.
type Tool struct {
	Type     string      `json:"type"` // always "function"
	Function FunctionDef `json:"function"`
}

// FunctionDef defines a callable function.
type FunctionDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Parameters is a JSON Schema object describing the function's parameters.
	Parameters any `json:"parameters,omitempty"`
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // always "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments from a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded arguments
}

// ToolChoice controls which tool (if any) the model will call.
// Use "none", "auto", "required", or a specific function name.
type ToolChoice any

// ToolChoiceAuto lets the model decide whether to call a tool.
const ToolChoiceAuto = "auto"

// ToolChoiceNone prevents the model from calling any tool.
const ToolChoiceNone = "none"

// ToolChoiceRequired forces the model to call at least one tool.
const ToolChoiceRequired = "required"

// SpecificToolChoice forces the model to call a specific function.
type SpecificToolChoice struct {
	Type     string               `json:"type"` // always "function"
	Function SpecificToolFunction `json:"function"`
}

// SpecificToolFunction names the function to force-call.
type SpecificToolFunction struct {
	Name string `json:"name"`
}

// Usage reports token consumption for a request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
