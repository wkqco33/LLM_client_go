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
