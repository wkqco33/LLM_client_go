package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/ollama"
	"github.com/wkqco33/LLM_client_go/openai"
	"github.com/wkqco33/LLM_client_go/retry"
)

// ─── 테스트 헬퍼 ──────────────────────────────────────────────

func testServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, func(ollama.Config) *openai.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, func(cfg ollama.Config) *openai.Client {
		cfg.BaseURL = srv.URL
		return ollama.New(cfg)
	}
}

// ─── 클라이언트 생성 ──────────────────────────────────────────

func TestNew_DefaultBaseURL(t *testing.T) {
	// 기본 URL 없이 생성 시 nil이 아니어야 함
	client := ollama.New(ollama.Config{})
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNew_CustomBaseURL(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	}))
	defer srv.Close()

	client := ollama.New(ollama.Config{BaseURL: srv.URL})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "llama3",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if !called {
		t.Error("expected request to reach custom BaseURL")
	}
}

func TestNew_SendsOllamaAPIKey(t *testing.T) {
	// Ollama는 API 키가 없지만 openai 클라이언트가 "ollama"를 Bearer로 전송
	srv, newClient := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer ollama" {
			t.Errorf("expected Authorization='Bearer ollama', got %q", auth)
		}
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	})
	_ = srv

	client := newClient(ollama.Config{})
	client.Complete(context.Background(), llm.ChatRequest{
		Model:    "llama3",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
}

func TestNew_WithTimeout(t *testing.T) {
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer slowSrv.Close()

	client := ollama.New(ollama.Config{
		BaseURL:     slowSrv.URL,
		Timeout:     50 * time.Millisecond,
		RetryPolicy: &retry.Policy{},
	})
	_, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "llama3",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// ─── 기능 동작 검증 ───────────────────────────────────────────

func TestComplete_ReturnsResponse(t *testing.T) {
	_, newClient := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(llm.ChatResponse{
			ID:    "ollama-cmpl-1",
			Model: "llama3",
			Choices: []llm.Choice{{
				Message:      llm.Message{Role: llm.RoleAssistant, Content: "Hello from Ollama!"},
				FinishReason: "stop",
			}},
		})
	})

	client := newClient(ollama.Config{})
	resp, err := client.Complete(context.Background(), llm.ChatRequest{
		Model:    "llama3",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].Message.Content != "Hello from Ollama!" {
		t.Errorf("unexpected content: %q", resp.Choices[0].Message.Content)
	}
}

func TestStream_CollectsContent(t *testing.T) {
	chunks := []string{
		`{"id":"s1","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
		`{"id":"s1","choices":[{"index":0,"delta":{"content":" Ollama"},"finish_reason":null}]}`,
		`{"id":"s1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`[DONE]`,
	}

	_, newClient := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		var sb strings.Builder
		for _, c := range chunks {
			sb.WriteString("data: " + c + "\n\n")
		}
		w.Write([]byte(sb.String()))
	})

	client := newClient(ollama.Config{})
	stream, err := client.Stream(context.Background(), llm.ChatRequest{
		Model:    "llama3",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
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
	if content != "Hello Ollama" {
		t.Errorf("expected 'Hello Ollama', got %q", content)
	}
}

func TestTokenCounter_NotNil(t *testing.T) {
	client := ollama.New(ollama.Config{})
	if client.TokenCounter("llama3") == nil {
		t.Error("expected non-nil TokenCounter")
	}
}
