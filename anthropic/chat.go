package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	llm "llm-client-go"
)

// ChatService provides access to the Anthropic Messages API.
type ChatService struct {
	client *Client
}

// messageRequest is the internal request format for Anthropic.
type messageRequest struct {
	Model         string         `json:"model"`
	Messages      []message      `json:"messages"`
	System        string         `json:"system,omitempty"`
	MaxTokens     int            `json:"max_tokens"`
	Temperature   *float64       `json:"temperature,omitempty"`
	TopP          *float64       `json:"top_p,omitempty"`
	StopSequences []string       `json:"stop_sequences,omitempty"`
	Tools         []tool         `json:"tools,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // can be string or []contentBlock
}

type contentBlock struct {
	Type      string     `json:"type"`
	Text      string     `json:"text,omitempty"`
	ID        string     `json:"id,omitempty"`           // for tool_use/tool_result
	Name      string     `json:"name,omitempty"`         // for tool_use
	Input     any        `json:"input,omitempty"`        // for tool_use
	ToolUseID string     `json:"tool_use_id,omitempty"`  // for tool_result
	Content   any        `json:"content,omitempty"`      // for tool_result
}

type tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

type messageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []contentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Usage        usage          `json:"usage"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Complete sends a non-streaming chat completion request.
func (s *ChatService) Complete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	antReq, err := s.toAnthropicRequest(req)
	if err != nil {
		return nil, err
	}
	antReq.Stream = false

	resp, err := s.client.do(ctx, http.MethodPost, "/messages", antReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	var result messageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}

	return s.fromAnthropicResponse(result), nil
}

func (s *ChatService) toAnthropicRequest(req llm.ChatRequest) (*messageRequest, error) {
	antReq := &messageRequest{
		Model:         req.Model,
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.Stop,
	}

	if antReq.MaxTokens == 0 {
		antReq.MaxTokens = 4096 // Anthropic requires max_tokens
	}

	for _, m := range req.Messages {
		if m.Role == llm.RoleSystem {
			antReq.System += m.Content // Concatenate system prompts
			continue
		}

		var antMsg message
		antMsg.Role = string(m.Role)
		
		// Map tool roles
		if m.Role == llm.RoleTool {
			antMsg.Role = "user" // Tool results are sent as "user" in Anthropic
			antMsg.Content = []contentBlock{{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}}
		} else if len(m.ToolCalls) > 0 {
			blocks := []contentBlock{}
			if m.Content != "" {
				blocks = append(blocks, contentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var input any
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				blocks = append(blocks, contentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			antMsg.Content = blocks
		} else {
			antMsg.Content = m.Content
		}
		
		antReq.Messages = append(antReq.Messages, antMsg)
	}

	for _, t := range req.Tools {
		antReq.Tools = append(antReq.Tools, tool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	return antReq, nil
}

func (s *ChatService) fromAnthropicResponse(antResp messageResponse) *llm.ChatResponse {
	resp := &llm.ChatResponse{
		ID:      antResp.ID,
		Object:  "chat.completion",
		Created: 0, // Anthropic doesn't provide this in the main response
		Model:   antResp.Model,
		Usage: llm.Usage{
			PromptTokens:     antResp.Usage.InputTokens,
			CompletionTokens: antResp.Usage.OutputTokens,
			TotalTokens:      antResp.Usage.InputTokens + antResp.Usage.OutputTokens,
		},
	}

	var content string
	var toolCalls []llm.ToolCall

	for _, b := range antResp.Content {
		if b.Type == "text" {
			content += b.Text
		} else if b.Type == "tool_use" {
			args, _ := json.Marshal(b.Input)
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   b.ID,
				Type: "function",
				Function: llm.FunctionCall{
					Name:      b.Name,
					Arguments: string(args),
				},
			})
		}
	}

	finishReason := antResp.StopReason
	if finishReason == "end_turn" {
		finishReason = "stop"
	} else if finishReason == "tool_use" {
		finishReason = "tool_calls"
	}

	resp.Choices = []llm.Choice{{
		Index: 0,
		Message: llm.Message{
			Role:      llm.RoleAssistant,
			Content:   content,
			ToolCalls: toolCalls,
		},
		FinishReason: finishReason,
	}}

	return resp
}

// 헬퍼 함수들 (OpenAI와 동일하게 제공)

func NewUserMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleUser, Content: content}
}
func NewSystemMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleSystem, Content: content}
}
func NewAssistantMessage(content string) llm.Message {
	return llm.Message{Role: llm.RoleAssistant, Content: content}
}
func NewToolResultMessage(toolCallID, content string) llm.Message {
	return llm.Message{Role: llm.RoleTool, ToolCallID: toolCallID, Content: content}
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
