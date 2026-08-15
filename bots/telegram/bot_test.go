package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/bots"
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

func TestNew_WithMockTelegramServer_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"TestBot","username":"test_bot"}}`))
	}))
	t.Cleanup(srv.Close)

	api, err := tgbotapi.NewBotAPIWithAPIEndpoint("dummy-token", srv.URL+"/bot%s/%s")
	if err != nil {
		t.Fatalf("failed to create bot api: %v", err)
	}

	bot := &Bot{
		api:      api,
		backend:  &mockBackend{},
		sessions: bots.NewSessionManager(),
	}
	if bot.api.Self.UserName != "test_bot" {
		t.Errorf("got username %q, want test_bot", bot.api.Self.UserName)
	}
}
