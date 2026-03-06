package azure_test

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
	"llm-client-go/azure"
)

// ─── 테스트 헬퍼 ──────────────────────────────────────────────

func testServer(t *testing.T, handler http.HandlerFunc) (*azure.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := azure.New(azure.Config{
		Endpoint: srv.URL,
		APIKey:   "test-azure-key",
	})
	return client, srv
}

func ptr[T any](v T) *T { return &v }

func makeSSE(chunks []string) string {
	var sb strings.Builder
	for _, c := range chunks {
		sb.WriteString("data: " + c + "\n\n")
	}
	return sb.String()
}

// ─── 클라이언트 생성 ──────────────────────────────────────────

func TestNew_DefaultAPIVersion(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RawQuery
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	}))
	defer srv.Close()

	client := azure.New(azure.Config{Endpoint: srv.URL, APIKey: "key"})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gpt4",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
	})

	if !strings.Contains(gotURL, "api-version=2024-02-01") {
		t.Errorf("expected default api-version=2024-02-01 in URL query, got %q", gotURL)
	}
}

func TestNew_WithAPIVersion_Option(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RawQuery
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	}))
	defer srv.Close()

	client := azure.New(azure.Config{Endpoint: srv.URL, APIKey: "key"},
		azure.WithAPIVersion("2025-01-01"),
	)
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gpt4",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
	})

	if !strings.Contains(gotURL, "api-version=2025-01-01") {
		t.Errorf("expected api-version=2025-01-01 in URL query, got %q", gotURL)
	}
}

func TestNew_EndpointTrailingSlash_Trimmed(t *testing.T) {
	// 엔드포인트 끝의 슬래시가 URL 중복 슬래시를 만들지 않아야 함
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	}))
	defer srv.Close()

	client := azure.New(azure.Config{Endpoint: srv.URL + "/", APIKey: "key"})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "my-deploy",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
	})

	if strings.Contains(gotPath, "//") {
		t.Errorf("URL should not contain double slashes: %q", gotPath)
	}
}

func TestNew_WithTimeout(t *testing.T) {
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(llm.ChatResponse{})
	}))
	defer slowSrv.Close()

	client := azure.New(azure.Config{Endpoint: slowSrv.URL, APIKey: "key"},
		azure.WithTimeout(50*time.Millisecond),
	)
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gpt4",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// ─── 인증 헤더 검증 ───────────────────────────────────────────

func TestComplete_SendsAPIKeyHeader(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("api-key")
		if apiKey != "test-azure-key" {
			t.Errorf("expected api-key='test-azure-key', got %q", apiKey)
		}
		// Azure는 Authorization 헤더를 사용하지 않음
		if r.Header.Get("Authorization") != "" {
			t.Error("Azure should NOT send Authorization header")
		}
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gpt4",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
}

func TestComplete_SendsContentTypeJSON(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type=application/json, got %q", r.Header.Get("Content-Type"))
		}
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gpt4",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
}

// ─── URL 구성 검증 ────────────────────────────────────────────

func TestComplete_DeploymentURL_Format(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	}))
	defer srv.Close()

	client := azure.New(azure.Config{Endpoint: srv.URL, APIKey: "key"})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "my-gpt4",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
	})

	expected := "/openai/deployments/my-gpt4/chat/completions"
	if gotPath != expected {
		t.Errorf("expected path=%q, got %q", expected, gotPath)
	}
}

// ─── Complete 응답 파싱 ────────────────────────────────────────

func TestComplete_ParsesResponseFields(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(llm.ChatResponse{
			ID:    "azure-cmpl-001",
			Model: "gpt-4",
			Choices: []llm.Choice{{
				Index:        0,
				Message:      llm.Message{Role: llm.RoleAssistant, Content: "Azure reply"},
				FinishReason: "stop",
			}},
			Usage: llm.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		})
	})

	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gpt4",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].Message.Content != "Azure reply" {
		t.Errorf("expected 'Azure reply', got %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 8 {
		t.Errorf("expected TotalTokens=8, got %d", resp.Usage.TotalTokens)
	}
}

func TestComplete_StreamForcedFalse(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if v, ok := body["stream"]; ok && v.(bool) {
			t.Error("stream must be false or omitted for Complete()")
		}
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "gpt4",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
		Stream:   true, // 강제 설정해도 Complete 내부에서 false로 재설정됨
	})
}

// ─── 에러 응답 처리 ───────────────────────────────────────────

func TestComplete_Error_401_Unauthorized(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
			"message": "Access denied", "code": "401",
		}})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gpt4", Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestComplete_Error_429_RateLimited(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "Rate limit"}})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gpt4", Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestComplete_Error_500_ServerError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "internal"}})
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gpt4", Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrServerError) {
		t.Errorf("expected ErrServerError, got %v", err)
	}
}

func TestComplete_Error_NonJSONFallback(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, "Bad Gateway")
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model: "gpt4", Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	var apiErr *llm.APIError
	if !llm.IsAPIError(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !strings.Contains(apiErr.Message, "Bad Gateway") {
		t.Errorf("expected raw body in message, got %q", apiErr.Message)
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
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gpt4", Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()
}

func TestStream_CollectsContent(t *testing.T) {
	chunks := []string{
		`{"id":"az1","choices":[{"index":0,"delta":{"role":"assistant","content":"Azure"},"finish_reason":null}]}`,
		`{"id":"az1","choices":[{"index":0,"delta":{"content":" stream"},"finish_reason":null}]}`,
		`{"id":"az1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`[DONE]`,
	}
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeSSE(chunks))
	})

	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gpt4", Messages: []llm.Message{azure.NewUserMessage("hi")},
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
	if content != "Azure stream" {
		t.Errorf("expected 'Azure stream', got %q", content)
	}
}

func TestStream_Close_PreventsNextCall(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	stream, _ := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gpt4", Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	stream.Close()

	_, err := stream.Next()
	if !errors.Is(err, llm.ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestStream_Error_ReturnsAPIError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "bad key"}})
	})
	_, err := client.Stream(context.Background(), llm.ChatRequest{
		Model: "gpt4", Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized from Stream, got %v", err)
	}
}

// ─── 메시지 / Tool 헬퍼 검증 ──────────────────────────────────

func TestMessageHelpers(t *testing.T) {
	cases := []struct {
		name    string
		msg     llm.Message
		role    llm.Role
		content string
	}{
		{"NewUserMessage", azure.NewUserMessage("hello"), llm.RoleUser, "hello"},
		{"NewSystemMessage", azure.NewSystemMessage("sys"), llm.RoleSystem, "sys"},
		{"NewAssistantMessage", azure.NewAssistantMessage("reply"), llm.RoleAssistant, "reply"},
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
	msg := azure.NewToolResultMessage("call_xyz", "42°C")
	if msg.Role != llm.RoleTool || msg.ToolCallID != "call_xyz" || msg.Content != "42°C" {
		t.Errorf("unexpected tool result message: %+v", msg)
	}
}

func TestNewTool_Structure(t *testing.T) {
	tool := azure.NewTool("calc", "Calculator", map[string]any{"type": "object"})
	if tool.Type != "function" || tool.Function.Name != "calc" {
		t.Errorf("unexpected tool structure: %+v", tool)
	}
}

func TestForceToolChoice(t *testing.T) {
	choice := azure.ForceToolChoice("specific_func")
	if choice.Type != "function" || choice.Function.Name != "specific_func" {
		t.Errorf("unexpected SpecificToolChoice: %+v", choice)
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
		Model:    "gpt4",
		Messages: []llm.Message{azure.NewUserMessage("hi")},
	})
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}
