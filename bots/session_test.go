package bots_test

import (
	"testing"

	llm "llm-client-go"
	"llm-client-go/bots"
)

func TestSessionManager_AppendAndGet(t *testing.T) {
	sm := bots.NewSessionManager()

	sm.Append("user1", llm.Message{Role: llm.RoleUser, Content: "Hello"})
	sm.Append("user1", llm.Message{Role: llm.RoleAssistant, Content: "Hi there"})

	history := sm.GetHistory("user1")
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	if history[0].Content != "Hello" {
		t.Errorf("expected 'Hello', got %q", history[0].Content)
	}
}

func TestSessionManager_SystemPrompt(t *testing.T) {
	sm := bots.NewSessionManager(
		bots.WithSystemPrompt("You are a pirate."),
	)

	// Before any messages, GetHistory should return just the system message.
	history := sm.GetHistory("user1")
	if len(history) != 1 {
		t.Fatalf("expected 1 message (system), got %d", len(history))
	}
	if history[0].Role != llm.RoleSystem {
		t.Errorf("expected system role, got %q", history[0].Role)
	}

	sm.Append("user1", llm.Message{Role: llm.RoleUser, Content: "Ahoy!"})
	history = sm.GetHistory("user1")

	// System message + user message
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	if history[0].Role != llm.RoleSystem {
		t.Error("first message should always be system")
	}
}

func TestSessionManager_MaxHistory(t *testing.T) {
	sm := bots.NewSessionManager(bots.WithMaxHistory(4))

	for i := range 6 {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		sm.Append("user1", llm.Message{Role: role, Content: "msg"})
	}

	history := sm.GetHistory("user1")
	if len(history) > 4 {
		t.Errorf("expected at most 4 messages, got %d", len(history))
	}
}

func TestSessionManager_MaxHistory_PreservesSystem(t *testing.T) {
	sm := bots.NewSessionManager(
		bots.WithSystemPrompt("System"),
		bots.WithMaxHistory(3), // 1 system + 2 conversation
	)

	for i := range 5 {
		sm.Append("u", llm.Message{Role: llm.RoleUser, Content: "msg"})
		_ = i
	}

	history := sm.GetHistory("u")
	if len(history) > 3 {
		t.Errorf("expected at most 3 messages, got %d", len(history))
	}
	if history[0].Role != llm.RoleSystem {
		t.Error("system message must be preserved after trimming")
	}
}

func TestSessionManager_Reset(t *testing.T) {
	sm := bots.NewSessionManager()

	sm.Append("user1", llm.Message{Role: llm.RoleUser, Content: "Hi"})
	sm.Reset("user1")

	history := sm.GetHistory("user1")
	if len(history) != 0 {
		t.Errorf("expected empty history after reset, got %d messages", len(history))
	}
}

func TestSessionManager_Isolation(t *testing.T) {
	sm := bots.NewSessionManager()

	sm.Append("alice", llm.Message{Role: llm.RoleUser, Content: "Alice msg"})
	sm.Append("bob", llm.Message{Role: llm.RoleUser, Content: "Bob msg"})

	alice := sm.GetHistory("alice")
	bob := sm.GetHistory("bob")

	if len(alice) != 1 || alice[0].Content != "Alice msg" {
		t.Errorf("alice history corrupted: %v", alice)
	}
	if len(bob) != 1 || bob[0].Content != "Bob msg" {
		t.Errorf("bob history corrupted: %v", bob)
	}
}
