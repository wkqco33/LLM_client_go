package gemini_test

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
	"llm-client-go/gemini"
)

// ─── 테스트 헬퍼 ──────────────────────────────────────────────

func testServer(t *testing.T, handler http.HandlerFunc) (*gemini.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := gemini.New(gemini.Config{
		APIKey:  "test-gemini-key",
		BaseURL: srv.URL,
	})
	return client, srv
}

func ptr[T any](v T) *T { return &v }

// Gemini 응답 픽스처
func makeTextResponse(text, finishReason string) map[string]any {
	return map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"role":  "model",
				"parts": []map[string]any{{"text": text}},
			},
			"finishReason": finishReason,
			"index":        0,
		}},
		"usageMetadata": map[string]any{
			"promptTokenCount":     10,
			"candidatesTokenCount": 5,
			"totalTokenCount":      15,
		},
	}
}

func makeSSE(events []string) string {
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString("data: " + e + "\n\n")
	}
	return sb.String()
}

// ─── 클라이언트 생성 ──────────────────────────────────────────

func TestNew_DefaultValues(t *testing.T) {
	c := gemini.New(gemini.Config{APIKey: "key"})
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.Chat == nil {
		t.Fatal("Chat service must be initialized")
	}
}

func TestNew_WithBaseURL(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(makeTextResponse("ok", "STOP"))
	}))
	defer srv.Close()

	client := gemini.New(gemini.Config{APIKey: "key"}, gemini.WithBaseURL(srv.URL))
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if !called {
		t.Error("WithBaseURL should redirect requests to the custom URL")
	}
}

func TestNew_WithTimeout(t *testing.T) {
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer slowSrv.Close()

	client := gemini.New(gemini.Config{
		APIKey:  "key",
		BaseURL: slowSrv.URL,
	}, gemini.WithTimeout(50*time.Millisecond))

	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// ─── URL 구조 검증 ────────────────────────────────────────────

func TestComplete_URLContainsModelAndAction(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(makeTextResponse("ok", "STOP"))
	}))
	defer srv.Close()

	client := gemini.New(gemini.Config{APIKey: "key", BaseURL: srv.URL})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})

	expected := "/models/gemini-1.5-pro:generateContent"
	if gotPath != expected {
		t.Errorf("expected path=%q, got %q", expected, gotPath)
	}
}

func TestComplete_URLContainsAPIKey(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		json.NewEncoder(w).Encode(makeTextResponse("ok", "STOP"))
	}))
	defer srv.Close()

	client := gemini.New(gemini.Config{APIKey: "my-api-key", BaseURL: srv.URL})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})

	if !strings.Contains(gotQuery, "key=my-api-key") {
		t.Errorf("expected key=my-api-key in URL query, got %q", gotQuery)
	}
}

// ─── 요청 바디 검증 ───────────────────────────────────────────

func TestComplete_SystemMessageSeparated(t *testing.T) {
	// system 메시지는 system_instruction 필드로 분리되어야 함
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		si, ok := body["system_instruction"].(map[string]any)
		if !ok {
			t.Fatal("expected system_instruction field")
		}
		parts := si["parts"].([]any)
		text := parts[0].(map[string]any)["text"].(string)
		if text != "You are helpful." {
			t.Errorf("expected system_instruction text='You are helpful.', got %q", text)
		}

		// contents 배열에 system 역할 항목이 없어야 함
		contents := body["contents"].([]any)
		for _, c := range contents {
			role := c.(map[string]any)["role"].(string)
			if role == "system" {
				t.Error("system message should NOT appear in contents array")
			}
		}

		json.NewEncoder(w).Encode(makeTextResponse("ok", "STOP"))
	})

	client.Complete(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro",
		Messages: []llm.Message{
			gemini.NewSystemMessage("You are helpful."),
			gemini.NewUserMessage("hi"),
		},
	})
}

func TestComplete_UserRoleMappedCorrectly(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		contents := body["contents"].([]any)
		if len(contents) == 0 {
			t.Fatal("expected contents to be non-empty")
		}
		role := contents[0].(map[string]any)["role"].(string)
		if role != "user" {
			t.Errorf("expected role='user', got %q", role)
		}

		json.NewEncoder(w).Encode(makeTextResponse("ok", "STOP"))
	})

	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("hello")},
	})
}

func TestComplete_AssistantRoleMappedToModel(t *testing.T) {
	// Gemini에서 assistant는 "model"로 매핑되어야 함
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		contents := body["contents"].([]any)
		var modelRole string
		for _, c := range contents {
			if c.(map[string]any)["role"].(string) == "model" {
				modelRole = "model"
			}
		}
		if modelRole != "model" {
			t.Error("expected assistant role to be mapped to 'model'")
		}

		json.NewEncoder(w).Encode(makeTextResponse("ok", "STOP"))
	})

	client.Complete(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro",
		Messages: []llm.Message{
			gemini.NewUserMessage("hi"),
			gemini.NewAssistantMessage("hello"),
			gemini.NewUserMessage("bye"),
		},
	})
}

func TestComplete_ToolsSentAsFunctionDeclarations(t *testing.T) {
	// Gemini 도구 형식: tools[0].function_declarations
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		tools, ok := body["tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Fatal("expected tools in request body")
		}
		decls := tools[0].(map[string]any)["function_declarations"].([]any)
		if len(decls) == 0 {
			t.Fatal("expected function_declarations")
		}
		decl := decls[0].(map[string]any)
		if decl["name"] != "get_weather" {
			t.Errorf("expected name='get_weather', got %v", decl["name"])
		}

		json.NewEncoder(w).Encode(makeTextResponse("ok", "STOP"))
	})

	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("weather?")},
		Tools: []llm.Tool{
			gemini.NewTool("get_weather", "Get weather", map[string]any{"type": "object"}),
		},
	})
}

func TestComplete_ToolResultSentAsFunctionResponse(t *testing.T) {
	// 도구 결과는 user role + functionResponse 파트로 전송
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		contents := body["contents"].([]any)
		last := contents[len(contents)-1].(map[string]any)
		if last["role"] != "user" {
			t.Errorf("tool result must be sent as 'user' role, got %q", last["role"])
		}
		parts := last["parts"].([]any)
		fr := parts[0].(map[string]any)["functionResponse"].(map[string]any)
		if fr["name"] != "get_weather" {
			t.Errorf("expected functionResponse.name='get_weather', got %v", fr["name"])
		}

		json.NewEncoder(w).Encode(makeTextResponse("done", "STOP"))
	})

	client.Complete(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro",
		Messages: []llm.Message{
			gemini.NewUserMessage("Weather?"),
			gemini.NewToolResultMessage("get_weather", "22°C"),
		},
	})
}

func TestComplete_GenerationConfig(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		gc, ok := body["generationConfig"].(map[string]any)
		if !ok {
			t.Fatal("expected generationConfig field")
		}
		if gc["maxOutputTokens"].(float64) != 512 {
			t.Errorf("expected maxOutputTokens=512, got %v", gc["maxOutputTokens"])
		}
		if gc["temperature"].(float64) != 0.7 {
			t.Errorf("expected temperature=0.7, got %v", gc["temperature"])
		}

		json.NewEncoder(w).Encode(makeTextResponse("ok", "STOP"))
	})

	client.Complete(context.Background(), llm.ChatRequest{
		Model:       "gemini-1.5-pro",
		Messages:    []llm.Message{gemini.NewUserMessage("hi")},
		MaxTokens:   512,
		Temperature: ptr(0.7),
	})
}

// ─── 응답 파싱 ────────────────────────────────────────────────

func TestComplete_ParsesTextResponse(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(makeTextResponse("Paris", "STOP"))
	})

	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("Capital of France?")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].Message.Content != "Paris" {
		t.Errorf("expected 'Paris', got %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("expected PromptTokens=10, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected TotalTokens=15, got %d", resp.Usage.TotalTokens)
	}
}

func TestComplete_FinishReason_StopMapped(t *testing.T) {
	// Gemini "STOP" → 공통 "stop"
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(makeTextResponse("ok", "STOP"))
	})
	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("expected 'stop' (mapped from 'STOP'), got %q", resp.Choices[0].FinishReason)
	}
}

func TestComplete_FinishReason_MaxTokensMapped(t *testing.T) {
	// Gemini "MAX_TOKENS" → 공통 "length"
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(makeTextResponse("ok", "MAX_TOKENS"))
	})
	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].FinishReason != "length" {
		t.Errorf("expected 'length' (mapped from 'MAX_TOKENS'), got %q", resp.Choices[0].FinishReason)
	}
}

func TestComplete_ToolCallResponse(t *testing.T) {
	// 함수 호출 응답 파싱
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"role": "model",
					"parts": []map[string]any{{
						"functionCall": map[string]any{
							"name": "get_weather",
							"args": map[string]any{"city": "Tokyo"},
						},
					}},
				},
				"finishReason": "STOP",
				"index":        0,
			}},
			"usageMetadata": map[string]any{"promptTokenCount": 20, "candidatesTokenCount": 10, "totalTokenCount": 30},
		})
	})

	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("Weather in Tokyo?")},
		Tools:    []llm.Tool{gemini.NewTool("get_weather", "Get weather", nil)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("expected FinishReason='tool_calls', got %q", resp.Choices[0].FinishReason)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.Function.Name != "get_weather" {
		t.Errorf("expected Name='get_weather', got %q", tc.Function.Name)
	}
	// Gemini는 ID가 없어 함수명을 ID로 사용
	if tc.ID != "get_weather" {
		t.Errorf("expected ID='get_weather' (function name), got %q", tc.ID)
	}
}

// ─── 에러 처리 ────────────────────────────────────────────────

func TestComplete_Error_401_Unauthorized(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": 401, "message": "API key not valid", "status": "UNAUTHENTICATED"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestComplete_Error_429_RateLimited(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": 429, "message": "Resource exhausted", "status": "RESOURCE_EXHAUSTED"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestComplete_Error_404_NotFound(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": 404, "message": "Model not found", "status": "NOT_FOUND"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gemini-invalid", Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestComplete_Error_400_BadRequest(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": 400, "message": "Bad parameter", "status": "INVALID_ARGUMENT"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestComplete_Error_403_Forbidden(t *testing.T) {
	// 403 Forbidden도 ErrUnauthorized로 처리
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": 403, "message": "Permission denied", "status": "PERMISSION_DENIED"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for 403, got %v", err)
	}
}

func TestComplete_Error_500_ServerError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": 500, "message": "Internal error", "status": "INTERNAL"},
		})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
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
		Model:    "gemini-1.5-pro",
		Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}

// ─── Stream 검증 ──────────────────────────────────────────────

func TestStream_URLContainsStreamAction(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSE([]string{
			`{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}]}`,
		}))
	}))
	defer srv.Close()

	client := gemini.New(gemini.Config{APIKey: "key", BaseURL: srv.URL})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()

	expected := "/models/gemini-1.5-pro:streamGenerateContent"
	if gotPath != expected {
		t.Errorf("expected stream path=%q, got %q", expected, gotPath)
	}
}

func TestStream_CollectsContent(t *testing.T) {
	events := []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]},"finishReason":""}],"usageMetadata":{}}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":", world"}]},"finishReason":""}],"usageMetadata":{}}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":""}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}`,
	}

	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSE(events))
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
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

func TestStream_ReturnsUsage(t *testing.T) {
	events := []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`,
	}

	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSE(events))
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
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
		t.Fatal("expected last chunk to have usage")
	}
	if lastChunk.Usage.TotalTokens != 7 {
		t.Errorf("expected TotalTokens=7, got %d", lastChunk.Usage.TotalTokens)
	}
}

func TestStream_FinishReasonMapped(t *testing.T) {
	events := []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"done"}]},"finishReason":"STOP"}]}`,
	}

	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSE(events))
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
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
		t.Errorf("expected finish_reason='stop' (from 'STOP'), got %q", finishReason)
	}
}

func TestStream_Close_PreventsNextCall(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSE([]string{
			`{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}]}`,
		}))
	})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
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
		fmt.Fprint(w, makeSSE([]string{
			`{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}]}`,
		}))
	})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
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
			"error": map[string]any{"code": 401, "message": "bad key", "status": "UNAUTHENTICATED"},
		})
	})
	_, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gemini-1.5-pro", Messages: []llm.Message{gemini.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized from Stream, got %v", err)
	}
}

// ─── Embeddings 미지원 검증 ───────────────────────────────────

func TestCreateEmbeddings_NotSupported(t *testing.T) {
	client := gemini.New(gemini.Config{APIKey: "key"})
	_, err := client.CreateEmbeddings(context.Background(), llm.EmbeddingRequest{
		Model: "text-embedding-004",
		Input: []string{"hello"},
	})
	if err == nil {
		t.Fatal("expected error for unsupported embeddings, got nil")
	}
}

// ─── TokenCounter 검증 ───────────────────────────────────────

func TestTokenCounter_ReturnsHeuristicCounter(t *testing.T) {
	client := gemini.New(gemini.Config{APIKey: "key"})
	counter := client.TokenCounter("gemini-1.5-pro")
	if counter == nil {
		t.Fatal("expected non-nil TokenCounter, got nil")
	}
	tc, ok := counter.(interface {
		Count(string) int
	})
	if !ok {
		t.Fatalf("expected token.Counter interface, got %T", counter)
	}
	if n := tc.Count("hello world"); n == 0 {
		t.Error("expected non-zero token count")
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
		{"NewUserMessage", gemini.NewUserMessage("hi user"), llm.RoleUser, "hi user"},
		{"NewSystemMessage", gemini.NewSystemMessage("be helpful"), llm.RoleSystem, "be helpful"},
		{"NewAssistantMessage", gemini.NewAssistantMessage("sure"), llm.RoleAssistant, "sure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.msg.Role != tc.role || tc.msg.Content != tc.content {
				t.Errorf("expected role=%q content=%q, got %+v", tc.role, tc.content, tc.msg)
			}
		})
	}
}

func TestNewToolResultMessage(t *testing.T) {
	msg := gemini.NewToolResultMessage("get_weather", "22°C")
	if msg.Role != llm.RoleTool {
		t.Errorf("expected role=tool, got %q", msg.Role)
	}
	if msg.ToolCallID != "get_weather" {
		t.Errorf("expected ToolCallID='get_weather', got %q", msg.ToolCallID)
	}
	if msg.Content != "22°C" {
		t.Errorf("expected Content='22°C', got %q", msg.Content)
	}
}

func TestNewTool_Structure(t *testing.T) {
	tool := gemini.NewTool("search", "Search the web", map[string]any{"type": "object"})
	if tool.Type != "function" {
		t.Errorf("expected Type='function', got %q", tool.Type)
	}
	if tool.Function.Name != "search" {
		t.Errorf("expected Name='search', got %q", tool.Function.Name)
	}
}

// ─── Function Calling 전체 흐름 ───────────────────────────────

func TestComplete_FunctionCalling_FullFlow(t *testing.T) {
	turnCount := 0
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		turnCount++
		if turnCount == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"candidates": []map[string]any{{
					"content": map[string]any{
						"role": "model",
						"parts": []map[string]any{{
							"functionCall": map[string]any{
								"name": "get_weather",
								"args": map[string]any{"city": "Tokyo"},
							},
						}},
					},
					"finishReason": "STOP",
					"index":        0,
				}},
				"usageMetadata": map[string]any{"totalTokenCount": 20},
			})
		} else {
			// 두 번째 요청에 functionResponse가 있어야 함
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			contents := body["contents"].([]any)
			last := contents[len(contents)-1].(map[string]any)
			if last["role"] != "user" {
				t.Errorf("tool result must be 'user' role, got %q", last["role"])
			}
			json.NewEncoder(w).Encode(makeTextResponse("Tokyo is 22°C.", "STOP"))
		}
	})

	msgs := []llm.Message{gemini.NewUserMessage("Weather in Tokyo?")}

	// Turn 1
	resp1, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: msgs,
		Tools:    []llm.Tool{gemini.NewTool("get_weather", "Get weather", nil)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp1.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("expected finish_reason=tool_calls, got %q", resp1.Choices[0].FinishReason)
	}

	msgs = append(msgs, resp1.Choices[0].Message)
	for _, tc := range resp1.Choices[0].Message.ToolCalls {
		msgs = append(msgs, gemini.NewToolResultMessage(tc.ID, "22°C, sunny"))
	}

	// Turn 2
	resp2, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gemini-1.5-pro",
		Messages: msgs,
		Tools:    []llm.Tool{gemini.NewTool("get_weather", "Get weather", nil)},
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
