package anthropic_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llm "llm-client-go"
	"llm-client-go/anthropic"
)

// ─── 테스트 헬퍼 ──────────────────────────────────────────────

func testServer(t *testing.T, handler http.HandlerFunc) (*anthropic.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := anthropic.New(anthropic.Config{
		APIKey:  "test-anthropic-key",
		BaseURL: srv.URL,
	})
	return client, srv
}

func ptr[T any](v T) *T { return &v }

// Anthropic 응답 픽스처 생성 헬퍼
func makeTextResponse(id, model, content, stopReason string) map[string]any {
	return map[string]any{
		"id":   id,
		"type": "message",
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": content},
		},
		"model":       model,
		"stop_reason": stopReason,
		"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
	}
}

func makeSSELines(events []string) string {
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString("data: " + e + "\n\n")
	}
	return sb.String()
}

// ─── 클라이언트 생성 ──────────────────────────────────────────

func TestNew_DefaultValues(t *testing.T) {
	c := anthropic.New(anthropic.Config{APIKey: "key"})
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.Chat == nil {
		t.Fatal("Chat service must be initialized")
	}
}

func TestNew_WithTimeout(t *testing.T) {
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer slowSrv.Close()

	client := anthropic.New(anthropic.Config{
		APIKey:  "key",
		BaseURL: slowSrv.URL,
	}, anthropic.WithTimeout(50*time.Millisecond))

	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestNew_WithBaseURL(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	}))
	defer srv.Close()

	client := anthropic.New(anthropic.Config{APIKey: "key"}, anthropic.WithBaseURL(srv.URL))
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if !called {
		t.Error("WithBaseURL should redirect requests to the custom URL")
	}
}

// ─── 요청 헤더 검증 ───────────────────────────────────────────

func TestComplete_SendsXAPIKeyHeader(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-anthropic-key" {
			t.Errorf("expected x-api-key='test-anthropic-key', got %q", r.Header.Get("x-api-key"))
		}
		// Bearer 토큰 방식이 아닌 x-api-key 방식
		if r.Header.Get("Authorization") != "" {
			t.Error("Anthropic should NOT send Authorization header")
		}
		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
}

func TestComplete_SendsAnthropicVersionHeader(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-version") == "" {
			t.Error("expected anthropic-version header to be set")
		}
		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
}

func TestComplete_SendsContentTypeJSON(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type=application/json, got %q", r.Header.Get("Content-Type"))
		}
		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
}

// ─── 요청 바디 검증 ───────────────────────────────────────────

func TestComplete_StreamForcedFalse(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if v, ok := body["stream"]; ok && v.(bool) {
			t.Error("stream must be false or omitted for Complete()")
		}
		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
}

func TestComplete_MaxTokensDefault(t *testing.T) {
	// MaxTokens=0 이면 기본값 4096이 설정되어야 함
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["max_tokens"].(float64) != 4096 {
			t.Errorf("expected max_tokens=4096 as default, got %v", body["max_tokens"])
		}
		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:     "claude-3-5-sonnet-20241022",
		Messages:  []llm.Message{anthropic.NewUserMessage("hi")},
		MaxTokens: 0, // 기본값 트리거
	})
}

func TestComplete_MaxTokensCustom(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["max_tokens"].(float64) != 1024 {
			t.Errorf("expected max_tokens=1024, got %v", body["max_tokens"])
		}
		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:     "claude-3-5-sonnet-20241022",
		Messages:  []llm.Message{anthropic.NewUserMessage("hi")},
		MaxTokens: 1024,
	})
}

func TestComplete_SystemMessageSeparated(t *testing.T) {
	// Anthropic은 system 메시지를 messages 배열이 아닌 별도 "system" 필드로 보냄
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		// system 필드에 시스템 프롬프트가 있어야 함
		if body["system"] != "You are helpful." {
			t.Errorf("expected system='You are helpful.', got %v", body["system"])
		}

		// messages 배열에 system 역할 메시지가 없어야 함
		msgs := body["messages"].([]any)
		for _, m := range msgs {
			msg := m.(map[string]any)
			if msg["role"] == "system" {
				t.Error("system message should NOT appear in messages array for Anthropic")
			}
		}

		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{
			anthropic.NewSystemMessage("You are helpful."),
			anthropic.NewUserMessage("hi"),
		},
	})
}

func TestComplete_OptionalFields(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["temperature"].(float64) != 0.7 {
			t.Errorf("expected temperature=0.7, got %v", body["temperature"])
		}
		stops, ok := body["stop_sequences"].([]any)
		if !ok || stops[0].(string) != "END" {
			t.Errorf("expected stop_sequences=['END'], got %v", body["stop_sequences"])
		}
		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:       "claude-3-5-sonnet-20241022",
		Messages:    []llm.Message{anthropic.NewUserMessage("hi")},
		Temperature: ptr(0.7),
		Stop:        []string{"END"},
	})
}

// ─── Complete 응답 파싱 ────────────────────────────────────────

func TestComplete_ParsesResponseFields(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg-abc123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "Paris"},
			},
			"model":       "claude-3-5-sonnet-20241022",
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 8, "output_tokens": 3},
		})
	})

	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("Capital of France?")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "msg-abc123" {
		t.Errorf("expected ID='msg-abc123', got %q", resp.ID)
	}
	if resp.Choices[0].Message.Content != "Paris" {
		t.Errorf("expected Content='Paris', got %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 8 {
		t.Errorf("expected PromptTokens=8, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 3 {
		t.Errorf("expected CompletionTokens=3, got %d", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 11 {
		t.Errorf("expected TotalTokens=11, got %d", resp.Usage.TotalTokens)
	}
}

func TestComplete_StopReason_EndTurn_MappedToStop(t *testing.T) {
	// Anthropic "end_turn" → 공통 "stop"으로 변환되어야 함
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "hello", "end_turn"))
	})

	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("expected FinishReason='stop' (mapped from 'end_turn'), got %q", resp.Choices[0].FinishReason)
	}
}

func TestComplete_StopReason_ToolUse_MappedToToolCalls(t *testing.T) {
	// Anthropic "tool_use" → 공통 "tool_calls"로 변환되어야 함
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg-tool",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{
					"type":  "tool_use",
					"id":    "toolu_01",
					"name":  "get_weather",
					"input": map[string]any{"city": "Tokyo"},
				},
			},
			"model":       "claude-3-5-sonnet-20241022",
			"stop_reason": "tool_use",
			"usage":       map[string]any{"input_tokens": 20, "output_tokens": 10},
		})
	})

	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("Weather in Tokyo?")},
		Tools: []llm.Tool{
			anthropic.NewTool("get_weather", "Get weather", map[string]any{"type": "object"}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("expected FinishReason='tool_calls' (mapped from 'tool_use'), got %q", resp.Choices[0].FinishReason)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.ID != "toolu_01" {
		t.Errorf("expected ToolCall.ID='toolu_01', got %q", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("expected ToolCall.Function.Name='get_weather', got %q", tc.Function.Name)
	}
}

func TestComplete_ToolCallWithTextAndToolUse(t *testing.T) {
	// 텍스트와 도구 호출이 함께 있는 경우
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg-mixed",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "Let me check the weather."},
				{
					"type":  "tool_use",
					"id":    "toolu_02",
					"name":  "get_weather",
					"input": map[string]any{"city": "Seoul"},
				},
			},
			"model":       "claude-3-5-sonnet-20241022",
			"stop_reason": "tool_use",
			"usage":       map[string]any{"input_tokens": 15, "output_tokens": 8},
		})
	})

	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("Weather in Seoul?")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content != "Let me check the weather." {
		t.Errorf("unexpected text content: %q", resp.Choices[0].Message.Content)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
}

// ─── 도구 요청 변환 검증 ──────────────────────────────────────

func TestComplete_ToolsSentAsInputSchema(t *testing.T) {
	// Anthropic은 OpenAI의 "parameters" 대신 "input_schema"를 사용
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		tools, ok := body["tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Fatal("expected tools in request body")
		}
		tool := tools[0].(map[string]any)
		if tool["name"] != "search" {
			t.Errorf("expected tool name='search', got %v", tool["name"])
		}
		if tool["input_schema"] == nil {
			t.Error("expected input_schema field in tool definition")
		}
		if tool["parameters"] != nil {
			t.Error("Anthropic should use 'input_schema', not 'parameters'")
		}

		json.NewEncoder(w).Encode(makeTextResponse("msg-1", "claude-3-5-sonnet-20241022", "ok", "end_turn"))
	})

	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("search something")},
		Tools: []llm.Tool{
			anthropic.NewTool("search", "Search the web", map[string]any{"type": "object"}),
		},
	})
}

func TestComplete_ToolResultSentAsUserRole(t *testing.T) {
	// Anthropic은 tool 결과를 "user" 역할의 tool_result 콘텐츠로 전송
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		msgs := body["messages"].([]any)
		// 마지막 메시지(tool result)를 확인
		lastMsg := msgs[len(msgs)-1].(map[string]any)
		if lastMsg["role"] != "user" {
			t.Errorf("tool result must be sent as 'user' role in Anthropic, got %q", lastMsg["role"])
		}

		content := lastMsg["content"].([]any)
		block := content[0].(map[string]any)
		if block["type"] != "tool_result" {
			t.Errorf("expected content block type='tool_result', got %q", block["type"])
		}
		if block["tool_use_id"] != "toolu_01" {
			t.Errorf("expected tool_use_id='toolu_01', got %v", block["tool_use_id"])
		}

		json.NewEncoder(w).Encode(makeTextResponse("msg-2", "claude-3-5-sonnet-20241022", "done", "end_turn"))
	})

	client.Complete(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{
			anthropic.NewUserMessage("Weather?"),
			anthropic.NewToolResultMessage("toolu_01", "22°C"),
		},
	})
}

// ─── 에러 응답 처리 ───────────────────────────────────────────

func TestComplete_Error_401_Unauthorized(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"type": "authentication_error", "message": "Invalid API key"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestComplete_Error_429_RateLimited(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"type": "rate_limit_error", "message": "Rate limit exceeded"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestComplete_Error_404_NotFound(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"type": "not_found_error", "message": "Model not found"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "claude-invalid", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestComplete_Error_400_BadRequest(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"type": "invalid_request_error", "message": "Bad parameter"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestComplete_Error_500_ServerError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"type": "api_error", "message": "Internal server error"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrServerError) {
		t.Errorf("expected ErrServerError, got %v", err)
	}
}

// ─── Context 취소 ─────────────────────────────────────────────

func TestComplete_ContextCancellation(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Complete(ctx, llm.ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}

// ─── Stream 검증 ──────────────────────────────────────────────

func TestStream_SendsStreamTrue(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if v, ok := body["stream"].(bool); !ok || !v {
			t.Error("stream must be true for Stream()")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSELines([]string{
			`{"type":"message_stop"}`,
		}))
	})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()
}

func TestStream_CollectsContent(t *testing.T) {
	events := []string{
		`{"type":"message_start","message":{"id":"msg-s1","type":"message","role":"assistant","content":[],"model":"claude-3-5-sonnet-20241022","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":", world"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}`,
		`{"type":"message_stop"}`,
	}

	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSELines(events))
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
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

func TestStream_MessageStart_ReturnsIDAndModel(t *testing.T) {
	events := []string{
		`{"type":"message_start","message":{"id":"msg-xyz","type":"message","role":"assistant","content":[],"model":"claude-3-opus-20240229","stop_reason":null,"usage":{"input_tokens":5,"output_tokens":0}}}`,
		`{"type":"message_stop"}`,
	}

	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSELines(events))
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "claude-3-opus-20240229", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	chunk, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if chunk == nil {
		t.Fatal("expected first chunk from message_start")
	}
	if chunk.ID != "msg-xyz" {
		t.Errorf("expected chunk.ID='msg-xyz', got %q", chunk.ID)
	}
	if chunk.Model != "claude-3-opus-20240229" {
		t.Errorf("expected chunk.Model='claude-3-opus-20240229', got %q", chunk.Model)
	}
}

func TestStream_MessageDelta_ReturnsUsage(t *testing.T) {
	events := []string{
		`{"type":"message_start","message":{"id":"msg-u1","type":"message","role":"assistant","content":[],"model":"claude-3-5-sonnet-20241022","stop_reason":null,"usage":{"input_tokens":8,"output_tokens":0}}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":8,"output_tokens":3}}`,
		`{"type":"message_stop"}`,
	}

	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSELines(events))
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var lastChunk *llm.ChatStreamChunk
	for {
		chunk, err := stream.Next()
		if err != nil {
			t.Fatal(err)
		}
		if chunk == nil {
			break
		}
		lastChunk = chunk
	}

	if lastChunk == nil || lastChunk.Usage == nil {
		t.Fatal("expected last chunk to have usage from message_delta")
	}
	if lastChunk.Usage.CompletionTokens != 3 {
		t.Errorf("expected CompletionTokens=3, got %d", lastChunk.Usage.CompletionTokens)
	}
}

func TestStream_MessageDelta_StopReason_MappedToStop(t *testing.T) {
	events := []string{
		`{"type":"message_start","message":{"id":"msg-s","type":"message","role":"assistant","content":[],"model":"claude-3-5-sonnet-20241022","stop_reason":null,"usage":{"input_tokens":5,"output_tokens":0}}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		`{"type":"message_stop"}`,
	}

	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSELines(events))
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var finishReason string
	for {
		chunk, err := stream.Next()
		if err != nil {
			t.Fatal(err)
		}
		if chunk == nil {
			break
		}
		for _, c := range chunk.Choices {
			if c.FinishReason != nil {
				finishReason = *c.FinishReason
			}
		}
	}

	if finishReason != "stop" {
		t.Errorf("expected finish_reason='stop' (mapped from 'end_turn'), got %q", finishReason)
	}
}

func TestStream_Close_PreventsNextCall(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSELines([]string{`{"type":"message_stop"}`}))
	})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()

	_, err = stream.Next()
	if !errors.Is(err, llm.ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed after Close(), got %v", err)
	}
}

func TestStream_DoubleClose_NoError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSELines([]string{`{"type":"message_stop"}`}))
	})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()
	if err := stream.Close(); err != nil {
		t.Errorf("second Close() should not return error, got %v", err)
	}
}

func TestStream_Error_ReturnsAPIError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"type": "authentication_error", "message": "bad key"},
		})
	})
	_, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: []llm.Message{anthropic.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized from Stream, got %v", err)
	}
}

// ─── TokenCounter 검증 ────────────────────────────────────────

func TestTokenCounter_ReturnsHeuristicCounter(t *testing.T) {
	client := anthropic.New(anthropic.Config{APIKey: "key"})
	counter := client.TokenCounter("claude-3-5-sonnet-20241022")
	if counter == nil {
		t.Fatal("expected non-nil TokenCounter, got nil")
	}
	tc, ok := counter.(interface {
		Count(string) int
		CountMessages([]llm.Message) int
	})
	if !ok {
		t.Fatalf("expected token.Counter interface, got %T", counter)
	}
	if n := tc.Count("hello world"); n == 0 {
		t.Error("expected non-zero token count for 'hello world'")
	}
}

// ─── Embeddings 미지원 검증 ───────────────────────────────────

func TestCreateEmbeddings_NotSupported(t *testing.T) {
	client := anthropic.New(anthropic.Config{APIKey: "key"})
	_, err := client.CreateEmbeddings(context.Background(), llm.EmbeddingRequest{
		Model: "some-model",
		Input: []string{"hello"},
	})
	if err == nil {
		t.Fatal("expected error for unsupported embeddings, got nil")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected 'not supported' in error message, got %q", err.Error())
	}
}

// ─── 메시지 / 도구 헬퍼 검증 ─────────────────────────────────

func TestMessageHelpers(t *testing.T) {
	cases := []struct {
		name    string
		msg     llm.Message
		role    llm.Role
		content string
	}{
		{"NewUserMessage", anthropic.NewUserMessage("hello user"), llm.RoleUser, "hello user"},
		{"NewSystemMessage", anthropic.NewSystemMessage("be helpful"), llm.RoleSystem, "be helpful"},
		{"NewAssistantMessage", anthropic.NewAssistantMessage("sure"), llm.RoleAssistant, "sure"},
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
	msg := anthropic.NewToolResultMessage("toolu_01", "22°C")
	if msg.Role != llm.RoleTool {
		t.Errorf("expected role=tool, got %q", msg.Role)
	}
	if msg.ToolCallID != "toolu_01" {
		t.Errorf("expected ToolCallID='toolu_01', got %q", msg.ToolCallID)
	}
	if msg.Content != "22°C" {
		t.Errorf("expected Content='22°C', got %q", msg.Content)
	}
}

func TestNewTool_Structure(t *testing.T) {
	params := map[string]any{"type": "object"}
	tool := anthropic.NewTool("search", "Search the web", params)
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

// ─── Function Calling 전체 흐름 ───────────────────────────────

func TestComplete_FunctionCalling_FullFlow(t *testing.T) {
	turnCount := 0
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		turnCount++
		if turnCount == 1 {
			// 1단계: tool_use 응답
			json.NewEncoder(w).Encode(map[string]any{
				"id":   "msg-turn1",
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "toolu_weather",
						"name":  "get_weather",
						"input": map[string]any{"city": "Tokyo"},
					},
				},
				"model":       "claude-3-5-sonnet-20241022",
				"stop_reason": "tool_use",
				"usage":       map[string]any{"input_tokens": 20, "output_tokens": 15},
			})
		} else {
			// 2단계: tool_result를 포함한 요청 후 최종 응답
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			msgs := body["messages"].([]any)
			// 마지막 메시지가 user 역할의 tool_result여야 함
			lastMsg := msgs[len(msgs)-1].(map[string]any)
			if lastMsg["role"] != "user" {
				t.Errorf("tool result must be sent as 'user' role, got %q", lastMsg["role"])
			}
			json.NewEncoder(w).Encode(makeTextResponse("msg-turn2", "claude-3-5-sonnet-20241022", "Tokyo is 22°C.", "end_turn"))
		}
	})

	weatherTool := anthropic.NewTool("get_weather", "Get weather", map[string]any{
		"type":       "object",
		"properties": map[string]any{"city": map[string]any{"type": "string"}},
	})
	msgs := []llm.Message{anthropic.NewUserMessage("Weather in Tokyo?")}

	// Turn 1
	resp1, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: msgs, Tools: []llm.Tool{weatherTool},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp1.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("expected finish_reason=tool_calls, got %q", resp1.Choices[0].FinishReason)
	}

	msgs = append(msgs, resp1.Choices[0].Message)
	for _, tc := range resp1.Choices[0].Message.ToolCalls {
		msgs = append(msgs, anthropic.NewToolResultMessage(tc.ID, "22°C, sunny"))
	}

	// Turn 2
	resp2, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "claude-3-5-sonnet-20241022", Messages: msgs, Tools: []llm.Tool{weatherTool},
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
