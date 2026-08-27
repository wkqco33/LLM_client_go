package bots_test

import (
	"testing"
	"time"
	"unicode/utf8"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/bots"
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

func TestSplitMessage_ASCII(t *testing.T) {
	text := "abcdefghij"
	chunks := bots.SplitMessage(text, 4)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "abcd" || chunks[1] != "efgh" || chunks[2] != "ij" {
		t.Errorf("unexpected chunks: %v", chunks)
	}
}

func TestSplitMessage_NoSplitNeeded(t *testing.T) {
	text := "short"
	chunks := bots.SplitMessage(text, 10)
	if len(chunks) != 1 || chunks[0] != "short" {
		t.Errorf("expected single chunk, got %v", chunks)
	}

	chunksZero := bots.SplitMessage(text, 0)
	if len(chunksZero) != 1 || chunksZero[0] != "short" {
		t.Errorf("expected single chunk when maxLen <= 0, got %v", chunksZero)
	}
}

func TestSplitMessage_MultibyteUTF8(t *testing.T) {
	text := "안녕하세요 반갑습니다" // 11 runes
	chunks := bots.SplitMessage(text, 4)
	// Must split by runes, not slicing mid-byte
	for i, chunk := range chunks {
		if !utf8.ValidString(chunk) {
			t.Errorf("chunk %d is invalid UTF-8: %q", i, chunk)
		}
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "안녕하세" || chunks[1] != "요 반갑" || chunks[2] != "습니다" {
		t.Errorf("unexpected rune chunks: %v", chunks)
	}
}

func TestSessionManager_TTL_And_Close(t *testing.T) {
	sm := bots.NewSessionManager(bots.WithTTL(20 * time.Millisecond))
	defer sm.Close()

	sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "hi"})
	history := sm.GetHistory("u1")
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}

	// Wait for TTL expiration
	time.Sleep(50 * time.Millisecond)

	historyAfter := sm.GetHistory("u1")
	if len(historyAfter) != 0 {
		t.Errorf("expected history to be expired, got %d messages", len(historyAfter))
	}

	// Multiple Close calls should be safe
	sm.Close()
	sm.Close()
}

func BenchmarkSplitMessage(b *testing.B) {
	text := "안녕하세요. 이것은 긴 메시지 분할 성능을 테스트하기 위한 샘플 텍스트입니다. 여러 청크로 나누어집니다."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bots.SplitMessage(text, 20)
	}
}
