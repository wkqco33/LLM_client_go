package llm_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	llm "github.com/wkqco33/LLM_client_go"
)

// ─── APIError ───────────────────────────────────────────────

func TestAPIError_ErrorString_WithCode(t *testing.T) {
	err := &llm.APIError{StatusCode: 401, Code: "invalid_api_key", Message: "Incorrect API key"}
	got := err.Error()
	if !strings.Contains(got, "401") {
		t.Errorf("expected status code in message, got: %q", got)
	}
	if !strings.Contains(got, "invalid_api_key") {
		t.Errorf("expected code in message, got: %q", got)
	}
	if !strings.Contains(got, "Incorrect API key") {
		t.Errorf("expected message text, got: %q", got)
	}
}

func TestAPIError_ErrorString_WithoutCode(t *testing.T) {
	err := &llm.APIError{StatusCode: 500, Message: "Internal server error"}
	got := err.Error()
	if !strings.Contains(got, "500") {
		t.Errorf("expected status code in message, got: %q", got)
	}
	// Code 없으면 "code=" 부분이 없어야 함
	if strings.Contains(got, "code=") {
		t.Errorf("unexpected 'code=' in message: %q", got)
	}
}

func TestAPIError_ImplementsError(t *testing.T) {
	var err error = &llm.APIError{StatusCode: 400, Message: "bad"}
	if err.Error() == "" {
		t.Error("APIError.Error() must return non-empty string")
	}
}

// ─── IsAPIError ─────────────────────────────────────────────

func TestIsAPIError_DirectError(t *testing.T) {
	original := &llm.APIError{StatusCode: 429, Message: "rate limited"}
	var target *llm.APIError
	if !llm.IsAPIError(original, &target) {
		t.Fatal("expected IsAPIError to return true for direct *APIError")
	}
	if target.StatusCode != 429 {
		t.Errorf("expected StatusCode=429, got %d", target.StatusCode)
	}
}

func TestIsAPIError_WrappedError(t *testing.T) {
	original := &llm.APIError{StatusCode: 401, Code: "invalid_api_key", Message: "bad key"}
	wrapped := fmt.Errorf("outer: %w", original)
	doubleWrapped := fmt.Errorf("outer2: %w", wrapped)

	var target *llm.APIError
	if !llm.IsAPIError(doubleWrapped, &target) {
		t.Fatal("expected IsAPIError to unwrap nested errors")
	}
	if target.Code != "invalid_api_key" {
		t.Errorf("expected Code='invalid_api_key', got %q", target.Code)
	}
}

func TestIsAPIError_NonAPIError(t *testing.T) {
	plain := errors.New("plain error")
	var target *llm.APIError
	if llm.IsAPIError(plain, &target) {
		t.Error("expected IsAPIError to return false for non-APIError")
	}
}

// ─── Sentinel errors ─────────────────────────────────────────

func TestSentinelErrors_ErrorsIs(t *testing.T) {
	sentinels := []struct {
		name     string
		sentinel error
	}{
		{"ErrUnauthorized", llm.ErrUnauthorized},
		{"ErrRateLimited", llm.ErrRateLimited},
		{"ErrNotFound", llm.ErrNotFound},
		{"ErrBadRequest", llm.ErrBadRequest},
		{"ErrServerError", llm.ErrServerError},
		{"ErrStreamClosed", llm.ErrStreamClosed},
	}

	for _, tc := range sentinels {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("wrapping: %w", tc.sentinel)
			if !errors.Is(wrapped, tc.sentinel) {
				t.Errorf("errors.Is failed for wrapped %s", tc.name)
			}
		})
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []error{
		llm.ErrUnauthorized,
		llm.ErrRateLimited,
		llm.ErrNotFound,
		llm.ErrBadRequest,
		llm.ErrServerError,
		llm.ErrStreamClosed,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %d and %d should be distinct", i, j)
			}
		}
	}
}

// ─── Message JSON ────────────────────────────────────────────

func TestMessage_JSON_UserMessage(t *testing.T) {
	msg := llm.Message{Role: llm.RoleUser, Content: "Hello"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var back llm.Message
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if back.Role != llm.RoleUser || back.Content != "Hello" {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}

func TestMessage_JSON_MultimodalMessage(t *testing.T) {
	msg := llm.NewUserMessageWithParts(
		llm.TextContent("Describe this image"),
		llm.ImageContent("data:image/png;base64,abc123"),
	)
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var body struct {
		Role    string            `json:"role"`
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("expected content array: %v; json=%s", err, data)
	}
	if body.Role != "user" || len(body.Content) != 2 {
		t.Fatalf("unexpected multimodal message: %s", data)
	}
	if !strings.Contains(string(data), `"type":"image_url"`) ||
		!strings.Contains(string(data), "data:image/png;base64,abc123") {
		t.Errorf("image part missing from JSON: %s", data)
	}

	var back llm.Message
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if back.Role != llm.RoleUser || len(back.ContentParts) != 2 {
		t.Fatalf("unexpected round-trip message: %+v", back)
	}
	if back.ContentParts[0].Text != "Describe this image" {
		t.Errorf("unexpected text part: %+v", back.ContentParts[0])
	}
	if back.ContentParts[1].ImageURL == nil || back.ContentParts[1].ImageURL.URL != "data:image/png;base64,abc123" {
		t.Errorf("unexpected image part: %+v", back.ContentParts[1])
	}
}

func TestImageContentData(t *testing.T) {
	part := llm.ImageContentData([]byte("image"), "image/png")
	if part.Type != "image_url" || part.ImageURL == nil {
		t.Fatalf("unexpected image part: %+v", part)
	}
	if part.ImageURL.URL != "data:image/png;base64,aW1hZ2U=" {
		t.Errorf("unexpected data URL: %q", part.ImageURL.URL)
	}
}

func TestMessage_JSON_OmitsEmptyFields(t *testing.T) {
	msg := llm.Message{Role: llm.RoleAssistant, Content: "Hi"}
	data, _ := json.Marshal(msg)
	s := string(data)
	for _, field := range []string{"tool_calls", "tool_call_id", "name"} {
		if strings.Contains(s, field) {
			t.Errorf("empty field %q should be omitted from JSON: %s", field, s)
		}
	}
}

func TestMessage_JSON_ToolCallID(t *testing.T) {
	msg := llm.Message{
		Role:       llm.RoleTool,
		Content:    "Tokyo: 22°C",
		ToolCallID: "call_abc",
	}
	data, _ := json.Marshal(msg)
	if !strings.Contains(string(data), "call_abc") {
		t.Errorf("tool_call_id should appear in JSON: %s", string(data))
	}

	var back llm.Message
	json.Unmarshal(data, &back)
	if back.ToolCallID != "call_abc" {
		t.Errorf("expected ToolCallID='call_abc', got %q", back.ToolCallID)
	}
}

func TestMessage_JSON_WithToolCalls(t *testing.T) {
	msg := llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "get_weather",
					Arguments: `{"city":"Seoul"}`,
				},
			},
		},
	}
	data, _ := json.Marshal(msg)
	var back llm.Message
	json.Unmarshal(data, &back)

	if len(back.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(back.ToolCalls))
	}
	if back.ToolCalls[0].ID != "call_1" {
		t.Errorf("expected ID='call_1', got %q", back.ToolCalls[0].ID)
	}
	if back.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected Name='get_weather', got %q", back.ToolCalls[0].Function.Name)
	}
}

// ─── Tool JSON ────────────────────────────────────────────────

func TestTool_JSON_RoundTrip(t *testing.T) {
	tool := llm.Tool{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        "my_func",
			Description: "does stuff",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{"type": "number"},
				},
			},
		},
	}
	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if !strings.Contains(string(data), `"type":"function"`) {
		t.Errorf("expected type=function in JSON: %s", string(data))
	}
	if !strings.Contains(string(data), "my_func") {
		t.Errorf("expected function name in JSON: %s", string(data))
	}
}

// ─── Role constants ───────────────────────────────────────────

func TestRoleConstants(t *testing.T) {
	cases := []struct {
		role llm.Role
		want string
	}{
		{llm.RoleSystem, "system"},
		{llm.RoleUser, "user"},
		{llm.RoleAssistant, "assistant"},
		{llm.RoleTool, "tool"},
	}
	for _, tc := range cases {
		if string(tc.role) != tc.want {
			t.Errorf("Role %v = %q, want %q", tc.role, tc.role, tc.want)
		}
	}
}

// ─── Usage ────────────────────────────────────────────────────

func TestUsage_JSON(t *testing.T) {
	u := llm.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	data, _ := json.Marshal(u)
	var back llm.Usage
	json.Unmarshal(data, &back)
	if back.PromptTokens != 10 || back.CompletionTokens != 20 || back.TotalTokens != 30 {
		t.Errorf("Usage round-trip failed: %+v", back)
	}
}

// ─── Message & Tool Helpers ───────────────────────────────────

func TestMessageHelpers(t *testing.T) {
	u := llm.NewUserMessage("user text")
	if u.Role != llm.RoleUser || u.Content != "user text" {
		t.Errorf("unexpected user message: %+v", u)
	}

	s := llm.NewSystemMessage("sys text")
	if s.Role != llm.RoleSystem || s.Content != "sys text" {
		t.Errorf("unexpected system message: %+v", s)
	}

	a := llm.NewAssistantMessage("asst text")
	if a.Role != llm.RoleAssistant || a.Content != "asst text" {
		t.Errorf("unexpected assistant message: %+v", a)
	}

	tr := llm.NewToolResultMessage("call_123", "tool result")
	if tr.Role != llm.RoleTool || tr.ToolCallID != "call_123" || tr.Content != "tool result" {
		t.Errorf("unexpected tool result message: %+v", tr)
	}
}

func TestToolHelpers(t *testing.T) {
	tool := llm.NewTool("lookup", "lookup data", map[string]any{"type": "object"})
	if tool.Type != "function" || tool.Function.Name != "lookup" || tool.Function.Description != "lookup data" {
		t.Errorf("unexpected tool: %+v", tool)
	}

	choice := llm.ForceToolChoice("lookup")
	if choice.Type != "function" || choice.Function.Name != "lookup" {
		t.Errorf("unexpected force tool choice: %+v", choice)
	}
}

func TestCollectToolCalls(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		got := llm.CollectToolCalls(nil)
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %d items", len(got))
		}
	})

	t.Run("SingleTool", func(t *testing.T) {
		deltas := []llm.ToolCallDelta{
			{Index: 0, ID: "call_1", Function: llm.FunctionCallDelta{Name: "get_"}},
			{Index: 0, Function: llm.FunctionCallDelta{Name: "weather", Arguments: `{"ci`}},
			{Index: 0, Function: llm.FunctionCallDelta{Arguments: `ty":"Seoul"}`}},
		}
		calls := llm.CollectToolCalls(deltas)
		if len(calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(calls))
		}
		if calls[0].ID != "call_1" || calls[0].Function.Name != "get_weather" || calls[0].Function.Arguments != `{"city":"Seoul"}` {
			t.Errorf("unexpected call: %+v", calls[0])
		}
	})

	t.Run("MultipleConcurrentTools", func(t *testing.T) {
		deltas := []llm.ToolCallDelta{
			{Index: 0, ID: "call_0", Function: llm.FunctionCallDelta{Name: "fn0", Arguments: "{"}},
			{Index: 1, ID: "call_1", Function: llm.FunctionCallDelta{Name: "fn1", Arguments: "{"}},
			{Index: 0, Function: llm.FunctionCallDelta{Arguments: `"a":1}`}},
			{Index: 1, Function: llm.FunctionCallDelta{Arguments: `"b":2}`}},
		}
		calls := llm.CollectToolCalls(deltas)
		if len(calls) != 2 {
			t.Fatalf("expected 2 calls, got %d", len(calls))
		}
		if calls[0].ID != "call_0" || calls[0].Function.Name != "fn0" || calls[0].Function.Arguments != `{"a":1}` {
			t.Errorf("unexpected call 0: %+v", calls[0])
		}
		if calls[1].ID != "call_1" || calls[1].Function.Name != "fn1" || calls[1].Function.Arguments != `{"b":2}` {
			t.Errorf("unexpected call 1: %+v", calls[1])
		}
	})
}

func TestEmbedding_JSON(t *testing.T) {
	req := llm.EmbeddingRequest{
		Model:          "text-embedding-3-small",
		Input:          []string{"hello", "world"},
		EncodingFormat: "float",
		Dimensions:     1536,
		User:           "user-1",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var backReq llm.EmbeddingRequest
	if err := json.Unmarshal(data, &backReq); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if backReq.Model != req.Model || len(backReq.Input) != 2 || backReq.Dimensions != 1536 {
		t.Errorf("round-trip mismatch: %+v", backReq)
	}

	resp := llm.EmbeddingResponse{
		Object: "list",
		Data: []llm.Embedding{
			{Object: "embedding", Embedding: []float32{0.1, 0.2, 0.3}, Index: 0},
		},
		Model: "text-embedding-3-small",
		Usage: llm.Usage{PromptTokens: 5, TotalTokens: 5},
	}
	respData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var backResp llm.EmbeddingResponse
	if err := json.Unmarshal(respData, &backResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(backResp.Data) != 1 || len(backResp.Data[0].Embedding) != 3 {
		t.Errorf("response round-trip mismatch: %+v", backResp)
	}
}

func BenchmarkCollectToolCalls(b *testing.B) {
	deltas := []llm.ToolCallDelta{
		{Index: 0, ID: "call_0", Function: llm.FunctionCallDelta{Name: "get_weather", Arguments: `{"city":`}},
		{Index: 1, ID: "call_1", Function: llm.FunctionCallDelta{Name: "search_db", Arguments: `{"query":`}},
		{Index: 0, Function: llm.FunctionCallDelta{Arguments: `"Seoul"}`}},
		{Index: 1, Function: llm.FunctionCallDelta{Arguments: `"restaurants"}`}},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = llm.CollectToolCalls(deltas)
	}
}
