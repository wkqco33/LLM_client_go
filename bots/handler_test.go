package bots_test

import (
	"context"
	"errors"
	"testing"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/bots"
)

// mockBackend is a test double for bots.Backend.
type mockBackend struct {
	reply string
	err   error
	calls [][]llm.Message
}

func (m *mockBackend) Complete(_ context.Context, messages []llm.Message) (string, error) {
	cp := make([]llm.Message, len(messages))
	copy(cp, messages)
	m.calls = append(m.calls, cp)
	return m.reply, m.err
}

func TestBackend_Complete_ForwardsMessages(t *testing.T) {
	mb := &mockBackend{reply: "I am fine, thanks!"}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "How are you?"},
	}

	reply, err := mb.Complete(context.Background(), msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "I am fine, thanks!" {
		t.Errorf("unexpected reply: %q", reply)
	}
	if len(mb.calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(mb.calls))
	}
}

func TestBackend_Complete_ReturnsError(t *testing.T) {
	want := errors.New("rate limited")
	mb := &mockBackend{err: want}

	_, err := mb.Complete(context.Background(), nil)
	if !errors.Is(err, want) {
		t.Errorf("expected rate limited error, got %v", err)
	}
}

func TestSessionManager_ConversationFlow(t *testing.T) {
	sm := bots.NewSessionManager(bots.WithSystemPrompt("Be concise."))
	mb := &mockBackend{reply: "42"}

	userID := "user-abc"

	// Simulate first user turn
	sm.Append(userID, llm.Message{Role: llm.RoleUser, Content: "What is 6*7?"})
	history := sm.GetHistory(userID)

	reply, err := mb.Complete(context.Background(), history)
	if err != nil {
		t.Fatal(err)
	}
	sm.Append(userID, llm.Message{Role: llm.RoleAssistant, Content: reply})

	// History should be: system + user + assistant
	history = sm.GetHistory(userID)
	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}

	// Verify messages passed to backend included system message
	passed := mb.calls[0]
	if passed[0].Role != llm.RoleSystem {
		t.Error("backend should receive system message as first item")
	}
}
