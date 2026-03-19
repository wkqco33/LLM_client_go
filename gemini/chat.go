package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	llm "llm-client-go"
)

// ChatService provides access to the Gemini generateContent API.
type ChatService struct {
	client *Client
}

// ─── Gemini Request Types ─────────────────────────────────────

type generateContentRequest struct {
	Contents          []gContent         `json:"contents"`
	SystemInstruction *systemInstruction `json:"system_instruction,omitempty"`
	Tools             []gTool            `json:"tools,omitempty"`
	GenerationConfig  *generationConfig  `json:"generationConfig,omitempty"`
}

type gContent struct {
	Role  string  `json:"role"`
	Parts []gPart `json:"parts"`
}

type gPart struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *gFunctionCall    `json:"functionCall,omitempty"`
	FunctionResponse *gFunctionResponse `json:"functionResponse,omitempty"`
}

type gFunctionCall struct {
	Name string `json:"name"`
	Args any    `json:"args"`
}

type gFunctionResponse struct {
	Name     string `json:"name"`
	Response any    `json:"response"`
}

type systemInstruction struct {
	Parts []gPart `json:"parts"`
}

type gTool struct {
	FunctionDeclarations []gFunctionDeclaration `json:"function_declarations"`
}

type gFunctionDeclaration struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type generationConfig struct {
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

// ─── Gemini Response Types ────────────────────────────────────

type generateContentResponse struct {
	Candidates    []gCandidate  `json:"candidates"`
	UsageMetadata gUsage        `json:"usageMetadata"`
}

type gCandidate struct {
	Content      gContent `json:"content"`
	FinishReason string   `json:"finishReason"`
	Index        int      `json:"index"`
}

type gUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// ─── Complete ─────────────────────────────────────────────────

// Complete sends a non-streaming chat completion request.
func (s *ChatService) Complete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	gReq, err := s.toGeminiRequest(req)
	if err != nil {
		return nil, err
	}

	url := s.client.modelURL(req.Model, "generateContent")
	resp, err := s.client.do(ctx, url, gReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	var result generateContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gemini: decode response: %w", err)
	}

	return s.fromGeminiResponse(req.Model, result), nil
}

// ─── Request Conversion ───────────────────────────────────────

func (s *ChatService) toGeminiRequest(req llm.ChatRequest) (*generateContentRequest, error) {
	gReq := &generateContentRequest{}

	// system 메시지는 system_instruction 필드로 분리
	var systemText string
	for _, m := range req.Messages {
		if m.Role == llm.RoleSystem {
			systemText += m.Content
		}
	}
	if systemText != "" {
		gReq.SystemInstruction = &systemInstruction{
			Parts: []gPart{{Text: systemText}},
		}
	}

	// 나머지 메시지 변환
	for _, m := range req.Messages {
		if m.Role == llm.RoleSystem {
			continue
		}
		c, err := toGeminiContent(m)
		if err != nil {
			return nil, err
		}
		gReq.Contents = append(gReq.Contents, c)
	}

	// 도구 변환: Gemini는 function_declarations 배열을 하나의 tools 항목으로 묶음
	if len(req.Tools) > 0 {
		var decls []gFunctionDeclaration
		for _, t := range req.Tools {
			decls = append(decls, gFunctionDeclaration{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			})
		}
		gReq.Tools = []gTool{{FunctionDeclarations: decls}}
	}

	// generationConfig
	if req.MaxTokens > 0 || req.Temperature != nil || req.TopP != nil || len(req.Stop) > 0 {
		gReq.GenerationConfig = &generationConfig{
			MaxOutputTokens: req.MaxTokens,
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			StopSequences:   req.Stop,
		}
	}

	return gReq, nil
}

func toGeminiContent(m llm.Message) (gContent, error) {
	c := gContent{}

	switch m.Role {
	case llm.RoleUser:
		c.Role = "user"
		c.Parts = []gPart{{Text: m.Content}}

	case llm.RoleAssistant:
		c.Role = "model"
		if len(m.ToolCalls) > 0 {
			if m.Content != "" {
				c.Parts = append(c.Parts, gPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var args any
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				c.Parts = append(c.Parts, gPart{
					FunctionCall: &gFunctionCall{
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}
		} else {
			c.Parts = []gPart{{Text: m.Content}}
		}

	case llm.RoleTool:
		// Gemini는 도구 결과를 user role의 functionResponse로 전송
		// ToolCallID에 함수 이름을 저장 (Gemini는 ID 대신 이름 사용)
		c.Role = "user"
		c.Parts = []gPart{{
			FunctionResponse: &gFunctionResponse{
				Name:     m.ToolCallID,
				Response: map[string]any{"content": m.Content},
			},
		}}
	}

	return c, nil
}

// ─── Response Conversion ──────────────────────────────────────

func (s *ChatService) fromGeminiResponse(model string, resp generateContentResponse) *llm.ChatResponse {
	result := &llm.ChatResponse{
		Model:  model,
		Object: "chat.completion",
		Usage: llm.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		},
	}

	for i, cand := range resp.Candidates {
		msg := llm.Message{Role: llm.RoleAssistant}
		hasToolCall := false

		for _, p := range cand.Content.Parts {
			if p.FunctionCall != nil {
				hasToolCall = true
				args, _ := json.Marshal(p.FunctionCall.Args)
				msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{
					ID:   p.FunctionCall.Name, // Gemini는 별도 ID 없음 → 함수명 사용
					Type: "function",
					Function: llm.FunctionCall{
						Name:      p.FunctionCall.Name,
						Arguments: string(args),
					},
				})
			} else if p.Text != "" {
				msg.Content += p.Text
			}
		}

		finishReason := mapFinishReason(cand.FinishReason)
		if hasToolCall {
			finishReason = "tool_calls"
		}

		result.Choices = append(result.Choices, llm.Choice{
			Index:        i,
			Message:      msg,
			FinishReason: finishReason,
		})
	}

	return result
}

// mapFinishReason maps Gemini finish reasons to the common format.
func mapFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION":
		return "content_filter"
	default:
		return reason
	}
}
