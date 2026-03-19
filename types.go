// Package llm provides common types shared across LLM provider implementations.
package llm

import (
	"context"
)

// Client is the interface for an LLM provider.
type Client interface {
	// Complete sends a non-streaming chat completion request.
	Complete(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// Stream starts a streaming chat completion request.
	Stream(ctx context.Context, req ChatRequest) (Stream, error)

	// CreateEmbeddings creates vector representations of the given inputs.
	CreateEmbeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)

	// TokenCounter returns a calculator for the specified model.
	TokenCounter(model string) any
}

// Stream is the interface for a streaming LLM response.
type Stream interface {
	// Next reads the next chunk from the stream.
	// Returns (nil, nil) when the stream is complete.
	Next() (*ChatStreamChunk, error)

	// Close releases resources held by the stream.
	Close() error
}

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

// ChatRequest is the input for a chat completion.
type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	TopP           *float64        `json:"top_p,omitempty"`
	N              int             `json:"n,omitempty"`
	Stop           []string        `json:"stop,omitempty"`
	Tools          []Tool          `json:"tools,omitempty"`
	ToolChoice     ToolChoice      `json:"tool_choice,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	StreamOptions  *StreamOptions  `json:"stream_options,omitempty"`
}

// EmbeddingRequest is the input for creating embeddings.
type EmbeddingRequest struct {
	// Model is the ID of the model to use (e.g., "text-embedding-3-small").
	Model string `json:"model"`
	// Input is the string or slice of strings to embed.
	Input []string `json:"input"`
	// EncodingFormat is the format to return the embeddings in ("float" or "base64").
	EncodingFormat string `json:"encoding_format,omitempty"`
	// Dimensions is the number of dimensions the resulting output embeddings should have.
	Dimensions int `json:"dimensions,omitempty"`
	// User is a unique identifier representing your end-user.
	User string `json:"user,omitempty"`
}

// EmbeddingResponse is the response from an embedding request.
type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

// Embedding represents a single vector embedding.
type Embedding struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// ResponseFormat defines the constrained output format.
type ResponseFormat struct {
	Type       string         `json:"type"`
	JSONSchema *JSONSchemaDef `json:"json_schema,omitempty"`
}

// JSONSchemaDef defines a schema for structured output.
type JSONSchemaDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema"`
	Strict      bool           `json:"strict,omitempty"`
}

// StreamOptions configures stream-specific behavior.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatResponse is the response from a non-streaming chat completion.
type ChatResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
}

// Choice is a single completion candidate.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ChatStreamChunk is a single SSE delta from the streaming response.
type ChatStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// StreamChoice is a single delta in a streaming chunk.
type StreamChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

// Delta carries the incremental content in a stream chunk.
type Delta struct {
	Role      Role            `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`
}

// ToolCallDelta carries the incremental tool call data in a stream chunk.
type ToolCallDelta struct {
	Index    int               `json:"index"`
	ID       string            `json:"id,omitempty"`
	Type     string            `json:"type,omitempty"`
	Function FunctionCallDelta `json:"function,omitempty"`
}

// FunctionCallDelta carries partial function call information.
type FunctionCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
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
	Parameters  any    `json:"parameters,omitempty"`
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
	Arguments string `json:"arguments"`
}

// ToolChoice controls which tool (if any) the model will call.
type ToolChoice any

const (
	ToolChoiceAuto     = "auto"
	ToolChoiceNone     = "none"
	ToolChoiceRequired = "required"
)

// SpecificToolChoice forces the model to call a specific function.
type SpecificToolChoice struct {
	Type     string               `json:"type"`
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
