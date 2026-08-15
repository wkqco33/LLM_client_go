package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	llm "github.com/wkqco33/LLM_client_go"
)

type mockBackend struct{}

func (m *mockBackend) Complete(ctx context.Context, messages []llm.Message) (string, error) {
	return "mock response", nil
}

func TestNew_MissingBotToken_Error(t *testing.T) {
	_, err := New(Config{
		AppToken: "xapp-dummy",
		Backend:  &mockBackend{},
	})
	if err == nil {
		t.Fatal("expected error for missing BotToken, got nil")
	}
}

func TestNew_MissingAppToken_Error(t *testing.T) {
	_, err := New(Config{
		BotToken: "xoxb-dummy",
		Backend:  &mockBackend{},
	})
	if err == nil {
		t.Fatal("expected error for missing AppToken, got nil")
	}
}

func TestNew_MissingBackend_Error(t *testing.T) {
	_, err := New(Config{
		BotToken: "xoxb-dummy",
		AppToken: "xapp-dummy",
	})
	if err == nil {
		t.Fatal("expected error for missing Backend, got nil")
	}
}

func TestNew_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"url":"https://example.com/","team":"TestTeam","user":"testbot","team_id":"T123","user_id":"U123"}`))
	}))
	t.Cleanup(srv.Close)

	bot, err := New(Config{
		BotToken: "xoxb-dummy",
		AppToken: "xapp-dummy",
		Backend:  &mockBackend{},
		APIURL:   srv.URL + "/",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bot == nil {
		t.Fatal("expected non-nil bot")
	}
	if bot.botUserID != "U123" {
		t.Errorf("got botUserID %q, want U123", bot.botUserID)
	}
}
