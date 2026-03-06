package openai_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llm "llm-client-go"
	"llm-client-go/openai"
)

// ─── 테스트 헬퍼 ──────────────────────────────────────────────

// testServer starts a mock HTTP server and returns an OpenAI client pointed at it.
func testServer(t *testing.T, handler http.HandlerFunc) (*openai.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return openai.New(openai.Config{APIKey: "test-key", BaseURL: srv.URL}), srv
}

func ptr[T any](v T) *T { return &v }

// ─── 클라이언트 생성 ──────────────────────────────────────────

func TestNew_DefaultBaseURL(t *testing.T) {
	// BaseURL을 비워두면 기본값이 사용되는지 검증 (실제 요청을 보내지 않고 생성만 확인)
	c := openai.New(openai.Config{APIKey: "key"})
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.Chat == nil {
		t.Fatal("Chat service must be initialized")
	}
}

func TestNew_WithBaseURL_Option(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(openai.ChatResponse{
			Choices: []openai.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	}))
	defer srv.Close()

	c := openai.New(openai.Config{APIKey: "key"}, openai.WithBaseURL(srv.URL))
	c.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if !called {
		t.Error("WithBaseURL option should redirect requests to the custom URL")
	}
}

func TestNew_WithTimeout_Option(t *testing.T) {
	// 타임아웃 설정 후 실제 타임아웃이 걸리는지 검증
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(openai.ChatResponse{})
	}))
	defer slowSrv.Close()

	c := openai.New(openai.Config{APIKey: "key", BaseURL: slowSrv.URL},
		openai.WithTimeout(50*time.Millisecond),
	)
	_, err := c.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// ─── 요청 헤더 / 바디 검증 ─────────────────────────────────────

func TestComplete_SendsAuthHeader(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("expected 'Bearer test-key', got %q", auth)
		}
		json.NewEncoder(w).Encode(openai.ChatResponse{
			Choices: []openai.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	})
	client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
}

func TestComplete_SendsContentTypeJSON(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type=application/json, got %q", ct)
		}
		json.NewEncoder(w).Encode(openai.ChatResponse{
			Choices: []openai.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	})
	client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
}

func TestComplete_RequestBody_StreamForcedFalse(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		// stream 필드가 없거나 false여야 함
		if v, ok := body["stream"]; ok && v.(bool) {
			t.Error("stream must be false or omitted for Complete()")
		}
		json.NewEncoder(w).Encode(openai.ChatResponse{
			Choices: []openai.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	})
	client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("hi")},
		Stream:   true, // 강제로 true 설정해도 Complete 내부에서 false로 재설정됨
	})
}

func TestComplete_RequestBody_ContainsModelAndMessages(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "gpt-4o" {
			t.Errorf("expected model='gpt-4o', got %v", body["model"])
		}
		msgs, ok := body["messages"].([]any)
		if !ok || len(msgs) == 0 {
			t.Error("messages field missing or empty in request body")
		}
		json.NewEncoder(w).Encode(openai.ChatResponse{
			Choices: []openai.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	})
	client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("hello")},
	})
}

func TestComplete_RequestBody_OptionalFields(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["max_tokens"].(float64) != 100 {
			t.Errorf("expected max_tokens=100, got %v", body["max_tokens"])
		}
		if body["temperature"].(float64) != 0.5 {
			t.Errorf("expected temperature=0.5, got %v", body["temperature"])
		}
		stops, ok := body["stop"].([]any)
		if !ok || stops[0].(string) != "END" {
			t.Errorf("expected stop=['END'], got %v", body["stop"])
		}
		json.NewEncoder(w).Encode(openai.ChatResponse{
			Choices: []openai.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	})
	client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:       "gpt-4o",
		Messages:    []llm.Message{openai.NewUserMessage("hi")},
		MaxTokens:   100,
		Temperature: ptr(0.5),
		Stop:        []string{"END"},
	})
}

// ─── Complete 응답 파싱 ────────────────────────────────────────

func TestComplete_ParsesResponseFields(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openai.ChatResponse{
			ID:    "chatcmpl-xyz",
			Model: "gpt-4o",
			Choices: []openai.Choice{
				{Index: 0, Message: llm.Message{Role: llm.RoleAssistant, Content: "Paris"}, FinishReason: "stop"},
			},
			Usage: llm.Usage{PromptTokens: 8, CompletionTokens: 3, TotalTokens: 11},
		})
	})

	resp, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("Capital of France?")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "chatcmpl-xyz" {
		t.Errorf("expected ID='chatcmpl-xyz', got %q", resp.ID)
	}
	if resp.Choices[0].Message.Content != "Paris" {
		t.Errorf("expected Content='Paris', got %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 11 {
		t.Errorf("expected TotalTokens=11, got %d", resp.Usage.TotalTokens)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("expected FinishReason='stop', got %q", resp.Choices[0].FinishReason)
	}
}

func TestComplete_MultipleChoices(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openai.ChatResponse{
			Choices: []openai.Choice{
				{Index: 0, Message: llm.Message{Content: "A"}, FinishReason: "stop"},
				{Index: 1, Message: llm.Message{Content: "B"}, FinishReason: "stop"},
			},
		})
	})

	resp, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", N: 2,
		Messages: []llm.Message{openai.NewUserMessage("pick one")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices) != 2 {
		t.Errorf("expected 2 choices, got %d", len(resp.Choices))
	}
}

// ─── 에러 응답 처리 ───────────────────────────────────────────

func TestComplete_Error_401_Unauthorized(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
			"message": "Incorrect API key", "code": "invalid_api_key", "type": "invalid_request_error",
		}})
	})
	_, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
	var apiErr *llm.APIError
	if !llm.IsAPIError(err, &apiErr) || apiErr.StatusCode != 401 {
		t.Errorf("expected *APIError with status 401, got %v", err)
	}
}

func TestComplete_Error_429_RateLimited(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
			"message": "Rate limit exceeded", "type": "rate_limit_error",
		}})
	})
	_, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestComplete_Error_404_NotFound(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "model not found"}})
	})
	_, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-999", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestComplete_Error_400_BadRequest(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "invalid param"}})
	})
	_, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestComplete_Error_500_ServerError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "internal error"}})
	})
	_, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrServerError) {
		t.Errorf("expected ErrServerError, got %v", err)
	}
}

func TestComplete_Error_NonJSONBody_Fallback(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
	})
	_, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
	var apiErr *llm.APIError
	if !llm.IsAPIError(err, &apiErr) {
		t.Errorf("expected *APIError even for non-JSON body")
	}
	if !strings.Contains(apiErr.Message, "Service Unavailable") {
		t.Errorf("expected raw body in message, got %q", apiErr.Message)
	}
}

func TestComplete_ContextCancellation(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		// 요청을 처리하지 않고 대기 — 컨텍스트 취소로 인한 중단을 테스트
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Chat.Complete(ctx, openai.ChatRequest{
		Model:    "gpt-4o",
		Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if err == nil {
		t.Error("expected error from context cancellation, got nil")
	}
}

// ─── Stream 검증 ──────────────────────────────────────────────

func makeSSE(chunks []string) string {
	var sb strings.Builder
	for _, c := range chunks {
		sb.WriteString("data: " + c + "\n\n")
	}
	return sb.String()
}

func TestStream_SendsStreamTrue(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if v, ok := body["stream"].(bool); !ok || !v {
			t.Error("stream must be true for Stream()")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\n\n"))
	})
	stream, err := client.Chat.Stream(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()
}

func TestStream_CollectsContent(t *testing.T) {
	chunks := []string{
		`{"id":"s1","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
		`{"id":"s1","choices":[{"index":0,"delta":{"content":", world"},"finish_reason":null}]}`,
		`{"id":"s1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`[DONE]`,
	}
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(makeSSE(chunks)))
	})

	stream, err := client.Chat.Stream(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var content string
	for {
		chunk, err := stream.Next()
		if err != nil {
			t.Fatal(err)
		}
		if chunk == nil {
			break
		}
		for _, c := range chunk.Choices {
			content += c.Delta.Content
		}
	}
	if content != "Hello, world" {
		t.Errorf("expected 'Hello, world', got %q", content)
	}
}

func TestStream_IgnoresNonDataLines(t *testing.T) {
	// SSE 스펙: 빈 줄, 주석(:로 시작), event: 줄은 무시해야 함
	payload := ": this is a comment\n" +
		"event: message\n" +
		`data: {"id":"s1","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}` + "\n\n" +
		"data: [DONE]\n\n"

	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(payload))
	})

	stream, _ := client.Chat.Stream(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	defer stream.Close()

	chunk, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if chunk == nil || chunk.Choices[0].Delta.Content != "ok" {
		t.Errorf("expected content='ok', got %v", chunk)
	}
}

func TestStream_IncludeUsage_FinalChunk(t *testing.T) {
	// 마지막 청크에 usage 필드가 있는 경우
	chunks := []string{
		`{"id":"s1","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`,
		`{"id":"s1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
		`[DONE]`,
	}
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(makeSSE(chunks)))
	})

	stream, _ := client.Chat.Stream(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
		StreamOptions: &openai.StreamOptions{IncludeUsage: true},
	})
	defer stream.Close()

	var lastChunk *openai.ChatStreamChunk
	for {
		chunk, _ := stream.Next()
		if chunk == nil {
			break
		}
		lastChunk = chunk
	}
	if lastChunk == nil || lastChunk.Usage == nil {
		t.Fatal("expected last chunk to have usage")
	}
	if lastChunk.Usage.TotalTokens != 7 {
		t.Errorf("expected TotalTokens=7, got %d", lastChunk.Usage.TotalTokens)
	}
}

func TestStream_Close_PreventsNextCall(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\n\n"))
	})
	stream, _ := client.Chat.Stream(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	stream.Close()

	_, err := stream.Next()
	if !errors.Is(err, llm.ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed after Close(), got %v", err)
	}
}

func TestStream_DoubleClose_NoError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\n\n"))
	})
	stream, _ := client.Chat.Stream(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	stream.Close()
	if err := stream.Close(); err != nil {
		t.Errorf("second Close() should not return error, got %v", err)
	}
}

func TestStream_Error_ReturnsAPIError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
			"message": "bad key", "code": "invalid_api_key",
		}})
	})
	_, err := client.Chat.Stream(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: []llm.Message{openai.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized from Stream, got %v", err)
	}
}

// ─── 메시지 헬퍼 검증 ─────────────────────────────────────────

func TestMessageHelpers(t *testing.T) {
	cases := []struct {
		name    string
		msg     llm.Message
		role    llm.Role
		content string
	}{
		{"NewUserMessage", openai.NewUserMessage("hi user"), llm.RoleUser, "hi user"},
		{"NewSystemMessage", openai.NewSystemMessage("be nice"), llm.RoleSystem, "be nice"},
		{"NewAssistantMessage", openai.NewAssistantMessage("sure"), llm.RoleAssistant, "sure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.msg.Role != tc.role {
				t.Errorf("expected role=%q, got %q", tc.role, tc.msg.Role)
			}
			if tc.msg.Content != tc.content {
				t.Errorf("expected content=%q, got %q", tc.content, tc.msg.Content)
			}
		})
	}
}

func TestNewToolResultMessage(t *testing.T) {
	msg := openai.NewToolResultMessage("call_123", "result data")
	if msg.Role != llm.RoleTool {
		t.Errorf("expected role=tool, got %q", msg.Role)
	}
	if msg.ToolCallID != "call_123" {
		t.Errorf("expected ToolCallID='call_123', got %q", msg.ToolCallID)
	}
	if msg.Content != "result data" {
		t.Errorf("expected Content='result data', got %q", msg.Content)
	}
}

// ─── Tool 헬퍼 검증 ───────────────────────────────────────────

func TestNewTool_Structure(t *testing.T) {
	params := map[string]any{"type": "object"}
	tool := openai.NewTool("search", "Search the web", params)

	if tool.Type != "function" {
		t.Errorf("expected Type='function', got %q", tool.Type)
	}
	if tool.Function.Name != "search" {
		t.Errorf("expected Name='search', got %q", tool.Function.Name)
	}
	if tool.Function.Description != "Search the web" {
		t.Errorf("expected Description='Search the web', got %q", tool.Function.Description)
	}
}

func TestForceToolChoice(t *testing.T) {
	choice := openai.ForceToolChoice("my_func")
	if choice.Type != "function" {
		t.Errorf("expected Type='function', got %q", choice.Type)
	}
	if choice.Function.Name != "my_func" {
		t.Errorf("expected Function.Name='my_func', got %q", choice.Function.Name)
	}
}

// ─── CollectToolCalls 검증 ────────────────────────────────────

func TestCollectToolCalls_SingleTool(t *testing.T) {
	deltas := []openai.ToolCallDelta{
		{Index: 0, ID: "call_1", Type: "function", Function: openai.FunctionCallDelta{Name: "get_w"}},
		{Index: 0, Function: openai.FunctionCallDelta{Name: "eather", Arguments: `{"ci`}},
		{Index: 0, Function: openai.FunctionCallDelta{Arguments: `ty":"Seoul"}`}},
	}
	calls := openai.CollectToolCalls(deltas)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "get_weather" {
		t.Errorf("expected Name='get_weather', got %q", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments != `{"city":"Seoul"}` {
		t.Errorf("unexpected arguments: %q", calls[0].Function.Arguments)
	}
}

func TestCollectToolCalls_MultipleConcurrentTools(t *testing.T) {
	deltas := []openai.ToolCallDelta{
		{Index: 0, ID: "call_a", Function: openai.FunctionCallDelta{Name: "func_a", Arguments: `{"x":1}`}},
		{Index: 1, ID: "call_b", Function: openai.FunctionCallDelta{Name: "func_b", Arguments: `{"y":2}`}},
	}
	calls := openai.CollectToolCalls(deltas)
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].ID != "call_a" || calls[1].ID != "call_b" {
		t.Errorf("unexpected call order: %v", calls)
	}
}

func TestCollectToolCalls_Empty(t *testing.T) {
	calls := openai.CollectToolCalls(nil)
	if len(calls) != 0 {
		t.Errorf("expected 0 calls for nil input, got %d", len(calls))
	}
}

// ─── Function Calling 전체 흐름 ───────────────────────────────

func TestComplete_FunctionCalling_FullFlow(t *testing.T) {
	// 1단계: 모델이 tool_calls 응답
	turnCount := 0
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		turnCount++
		if turnCount == 1 {
			json.NewEncoder(w).Encode(openai.ChatResponse{
				Choices: []openai.Choice{{
					Index: 0,
					Message: llm.Message{
						Role: llm.RoleAssistant,
						ToolCalls: []llm.ToolCall{{
							ID:   "call_weather",
							Type: "function",
							Function: llm.FunctionCall{
								Name:      "get_weather",
								Arguments: `{"city":"Tokyo"}`,
							},
						}},
					},
					FinishReason: "tool_calls",
				}},
			})
		} else {
			// 2단계: tool 결과를 받고 최종 응답
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			msgs := body["messages"].([]any)
			// 마지막 메시지가 tool role이어야 함
			last := msgs[len(msgs)-1].(map[string]any)
			if last["role"].(string) != "tool" {
				t.Errorf("last message should have role=tool, got %q", last["role"])
			}
			json.NewEncoder(w).Encode(openai.ChatResponse{
				Choices: []openai.Choice{{
					Message:      llm.Message{Role: llm.RoleAssistant, Content: "Tokyo is 22°C."},
					FinishReason: "stop",
				}},
			})
		}
	})

	weatherTool := openai.NewTool("get_weather", "Get weather", map[string]any{
		"type":       "object",
		"properties": map[string]any{"city": map[string]any{"type": "string"}},
		"required":   []string{"city"},
	})
	msgs := []llm.Message{openai.NewUserMessage("Weather in Tokyo?")}

	// Turn 1
	resp1, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: msgs, Tools: []llm.Tool{weatherTool},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp1.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("expected finish_reason=tool_calls, got %q", resp1.Choices[0].FinishReason)
	}

	msgs = append(msgs, resp1.Choices[0].Message)
	for _, tc := range resp1.Choices[0].Message.ToolCalls {
		msgs = append(msgs, openai.NewToolResultMessage(tc.ID, "22°C, sunny"))
	}

	// Turn 2
	resp2, err := client.Chat.Complete(context.Background(), openai.ChatRequest{
		Model: "gpt-4o", Messages: msgs, Tools: []llm.Tool{weatherTool},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Choices[0].Message.Content != "Tokyo is 22°C." {
		t.Errorf("unexpected final reply: %q", resp2.Choices[0].Message.Content)
	}
	if turnCount != 2 {
		t.Errorf("expected exactly 2 API calls, got %d", turnCount)
	}
}
