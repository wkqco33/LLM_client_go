package discord

import (
	"context"
	"testing"

	llm "github.com/wkqco33/LLM_client_go"
)

type mockBackend struct{}

func (m *mockBackend) Complete(ctx context.Context, messages []llm.Message) (string, error) {
	return "mock response", nil
}

func TestNew_MissingToken_Error(t *testing.T) {
	_, err := New(Config{
		Backend: &mockBackend{},
	})
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}

func TestNew_MissingBackend_Error(t *testing.T) {
	_, err := New(Config{
		Token: "dummy-token",
	})
	if err == nil {
		t.Fatal("expected error for missing backend, got nil")
	}
}

func TestNew_Success(t *testing.T) {
	bot, err := New(Config{
		Token:   "dummy-token",
		Backend: &mockBackend{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bot == nil {
		t.Fatal("expected non-nil bot")
	}
}
