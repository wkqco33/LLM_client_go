package bots_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	llm "llm-client-go"
	"llm-client-go/bots"
	"llm-client-go/openai"
)

// ─── SessionManager: 기본 동작 ────────────────────────────────

func TestSession_NewUser_EmptyHistory(t *testing.T) {
	sm := bots.NewSessionManager()
	history := sm.GetHistory("unknown-user")
	if len(history) != 0 {
		t.Errorf("new user should have empty history, got %d messages", len(history))
	}
}

func TestSession_Append_And_Get(t *testing.T) {
	sm := bots.NewSessionManager()
	sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "Hello"})
	sm.Append("u1", llm.Message{Role: llm.RoleAssistant, Content: "Hi there"})

	h := sm.GetHistory("u1")
	if len(h) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(h))
	}
	if h[0].Content != "Hello" || h[1].Content != "Hi there" {
		t.Errorf("unexpected message contents: %v", h)
	}
}

func TestSession_GetHistory_ReturnsCopy(t *testing.T) {
	// GetHistory가 슬라이스 복사본을 반환하여 외부 수정이 내부 상태에 영향을 주지 않아야 함
	sm := bots.NewSessionManager()
	sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "original"})

	h := sm.GetHistory("u1")
	h[0].Content = "tampered" // 외부에서 수정

	h2 := sm.GetHistory("u1")
	if h2[0].Content != "original" {
		t.Error("GetHistory should return a copy; internal state was mutated")
	}
}

// ─── SessionManager: 시스템 프롬프트 ──────────────────────────

func TestSession_SystemPrompt_PrependedOnFirstGet(t *testing.T) {
	sm := bots.NewSessionManager(bots.WithSystemPrompt("Be helpful"))
	h := sm.GetHistory("u1")
	if len(h) != 1 || h[0].Role != llm.RoleSystem {
		t.Fatalf("expected 1 system message, got %v", h)
	}
	if h[0].Content != "Be helpful" {
		t.Errorf("unexpected system message: %q", h[0].Content)
	}
}

func TestSession_SystemPrompt_AlwaysFirst(t *testing.T) {
	sm := bots.NewSessionManager(bots.WithSystemPrompt("System"))
	sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "msg1"})
	sm.Append("u1", llm.Message{Role: llm.RoleAssistant, Content: "reply1"})

	h := sm.GetHistory("u1")
	if h[0].Role != llm.RoleSystem {
		t.Errorf("system message should always be first, got role=%q", h[0].Role)
	}
}

func TestSession_SystemPrompt_RestoredAfterReset(t *testing.T) {
	sm := bots.NewSessionManager(bots.WithSystemPrompt("Prompt"))
	sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "hi"})
	sm.Reset("u1")

	// 리셋 후 새로 Append 하면 시스템 프롬프트가 다시 앞에 와야 함
	sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "hello again"})
	h := sm.GetHistory("u1")
	if h[0].Role != llm.RoleSystem {
		t.Errorf("system prompt should be prepended after reset, got role=%q", h[0].Role)
	}
}

// ─── SessionManager: MaxHistory ───────────────────────────────

func TestSession_MaxHistory_Trims(t *testing.T) {
	sm := bots.NewSessionManager(bots.WithMaxHistory(4))
	for i := range 8 {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		sm.Append("u1", llm.Message{Role: role, Content: "msg"})
	}
	if len(sm.GetHistory("u1")) > 4 {
		t.Errorf("history should not exceed MaxHistory=4, got %d", len(sm.GetHistory("u1")))
	}
}

func TestSession_MaxHistory_KeepsNewest(t *testing.T) {
	sm := bots.NewSessionManager(bots.WithMaxHistory(3))
	for i := range 5 {
		sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: string(rune('A' + i))})
	}
	h := sm.GetHistory("u1")
	// MaxHistory=3, 최신 3개(C,D,E)가 남아야 함
	last := h[len(h)-1].Content
	if last != "E" {
		t.Errorf("newest message should be 'E', got %q", last)
	}
}

func TestSession_MaxHistory_PreservesSystemMessage(t *testing.T) {
	sm := bots.NewSessionManager(
		bots.WithSystemPrompt("System"),
		bots.WithMaxHistory(3), // system + 2 conversation
	)
	for i := range 6 {
		sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "msg"})
		_ = i
	}
	h := sm.GetHistory("u1")
	if len(h) > 3 {
		t.Errorf("expected at most 3 messages, got %d", len(h))
	}
	if h[0].Role != llm.RoleSystem {
		t.Error("system message must survive trimming")
	}
}

func TestSession_MaxHistory_Zero_Unlimited(t *testing.T) {
	sm := bots.NewSessionManager(bots.WithMaxHistory(0))
	for i := range 100 {
		sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "x"})
		_ = i
	}
	if len(sm.GetHistory("u1")) != 100 {
		t.Errorf("MaxHistory=0 means unlimited, expected 100, got %d", len(sm.GetHistory("u1")))
	}
}

func TestSession_MaxHistory_ExactBoundary(t *testing.T) {
	// MaxHistory=N일 때 정확히 N개의 메시지가 있으면 트리밍이 일어나지 않아야 함
	sm := bots.NewSessionManager(bots.WithMaxHistory(3))
	for i := range 3 {
		sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "m"})
		_ = i
	}
	if len(sm.GetHistory("u1")) != 3 {
		t.Errorf("expected exactly 3 messages at boundary, got %d", len(sm.GetHistory("u1")))
	}
}

// ─── SessionManager: Reset ────────────────────────────────────

func TestSession_Reset_ClearsHistory(t *testing.T) {
	sm := bots.NewSessionManager()
	sm.Append("u1", llm.Message{Role: llm.RoleUser, Content: "hi"})
	sm.Reset("u1")
	if len(sm.GetHistory("u1")) != 0 {
		t.Error("history should be empty after Reset")
	}
}

func TestSession_Reset_NonExistentUser_NoError(t *testing.T) {
	sm := bots.NewSessionManager()
	// 존재하지 않는 유저를 리셋해도 패닉이 없어야 함
	sm.Reset("ghost")
}

func TestSession_Reset_MultipleUsers_Independent(t *testing.T) {
	sm := bots.NewSessionManager()
	sm.Append("alice", llm.Message{Role: llm.RoleUser, Content: "alice"})
	sm.Append("bob", llm.Message{Role: llm.RoleUser, Content: "bob"})

	sm.Reset("alice")

	if len(sm.GetHistory("alice")) != 0 {
		t.Error("alice should have empty history")
	}
	if len(sm.GetHistory("bob")) != 1 {
		t.Error("bob should still have 1 message")
	}
}

// ─── SessionManager: 유저 격리 ───────────────────────────────

func TestSession_UserIsolation(t *testing.T) {
	sm := bots.NewSessionManager()
	sm.Append("alice", llm.Message{Role: llm.RoleUser, Content: "I'm Alice"})
	sm.Append("bob", llm.Message{Role: llm.RoleUser, Content: "I'm Bob"})
	sm.Append("charlie", llm.Message{Role: llm.RoleUser, Content: "I'm Charlie"})

	for _, tc := range []struct{ user, want string }{
		{"alice", "I'm Alice"},
		{"bob", "I'm Bob"},
		{"charlie", "I'm Charlie"},
	} {
		h := sm.GetHistory(tc.user)
		if len(h) != 1 || h[0].Content != tc.want {
			t.Errorf("%s: expected %q, got %v", tc.user, tc.want, h)
		}
	}
}

// ─── SessionManager: 동시성 ───────────────────────────────────

func TestSession_ConcurrentAccess_NoDataRace(t *testing.T) {
	// -race 플래그와 함께 실행 시 데이터 레이스가 없어야 함
	sm := bots.NewSessionManager(bots.WithSystemPrompt("System"), bots.WithMaxHistory(10))
	users := []string{"u1", "u2", "u3", "u4", "u5"}

	var wg sync.WaitGroup
	for _, uid := range users {
		for range 20 {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				sm.Append(id, llm.Message{Role: llm.RoleUser, Content: "msg"})
				sm.GetHistory(id)
			}(uid)
		}
	}
	// 일부 리셋 동시 실행
	for _, uid := range users[:2] {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sm.Reset(id)
		}(uid)
	}
	wg.Wait()
}

// ─── OpenAIBackend ────────────────────────────────────────────

func TestOpenAIBackend_Complete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{
				Message:      llm.Message{Role: llm.RoleAssistant, Content: "42"},
				FinishReason: "stop",
			}},
		})
	}))
	defer srv.Close()

	backend := bots.NewOpenAIBackend("test-key", "gpt-4o", openai.WithBaseURL(srv.URL))
	reply, err := backend.Complete(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "What is 6*7?"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "42" {
		t.Errorf("expected '42', got %q", reply)
	}
}

func TestOpenAIBackend_Complete_ForwardsModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "gpt-4o-mini" {
			t.Errorf("expected model='gpt-4o-mini', got %v", body["model"])
		}
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	}))
	defer srv.Close()

	backend := bots.NewOpenAIBackend("key", "gpt-4o-mini", openai.WithBaseURL(srv.URL))
	backend.Complete(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "hi"}})
}

func TestOpenAIBackend_Complete_WrapsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
			"message": "bad key", "code": "invalid_api_key",
		}})
	}))
	defer srv.Close()

	backend := bots.NewOpenAIBackend("bad-key", "gpt-4o", openai.WithBaseURL(srv.URL))
	_, err := backend.Complete(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "hi"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, llm.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized wrapped in backend error, got %v", err)
	}
}

func TestOpenAIBackend_Complete_EmptyChoices_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.Choice{}})
	}))
	defer srv.Close()

	backend := bots.NewOpenAIBackend("key", "gpt-4o", openai.WithBaseURL(srv.URL))
	_, err := backend.Complete(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "hi"}})
	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
}

func TestOpenAIBackend_Complete_FullConversation(t *testing.T) {
	// Backend가 전체 메시지 히스토리를 그대로 전달하는지 검증
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		msgs := body["messages"].([]any)
		if len(msgs) != 3 {
			t.Errorf("expected 3 messages in request, got %d", len(msgs))
		}
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{Message: llm.Message{Content: "ok"}, FinishReason: "stop"}},
		})
	}))
	defer srv.Close()

	backend := bots.NewOpenAIBackend("key", "gpt-4o", openai.WithBaseURL(srv.URL))
	backend.Complete(context.Background(), []llm.Message{
		{Role: llm.RoleSystem, Content: "system"},
		{Role: llm.RoleUser, Content: "user"},
		{Role: llm.RoleAssistant, Content: "assistant"},
	})
}

// ─── OpenAIBackend + SessionManager 통합 ─────────────────────

func TestOpenAIBackend_WithSession_ConversationFlow(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		msgs := body["messages"].([]any)
		// 두 번째 호출에는 이전 대화가 포함되어야 함
		if callCount == 2 && len(msgs) < 3 {
			t.Errorf("second call should include conversation history, got %d msgs", len(msgs))
		}
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{{
				Message:      llm.Message{Role: llm.RoleAssistant, Content: "reply"},
				FinishReason: "stop",
			}},
		})
	}))
	defer srv.Close()

	sm := bots.NewSessionManager(bots.WithSystemPrompt("Be helpful"))
	backend := bots.NewOpenAIBackend("key", "gpt-4o", openai.WithBaseURL(srv.URL))
	userID := "test-user"

	// Turn 1
	sm.Append(userID, llm.Message{Role: llm.RoleUser, Content: "Hello"})
	reply, _ := backend.Complete(context.Background(), sm.GetHistory(userID))
	sm.Append(userID, llm.Message{Role: llm.RoleAssistant, Content: reply})

	// Turn 2
	sm.Append(userID, llm.Message{Role: llm.RoleUser, Content: "How are you?"})
	backend.Complete(context.Background(), sm.GetHistory(userID))

	if callCount != 2 {
		t.Errorf("expected 2 backend calls, got %d", callCount)
	}
	// system + user1 + assistant1 + user2 = 4
	if len(sm.GetHistory(userID)) != 4 {
		t.Errorf("expected 4 messages in session, got %d", len(sm.GetHistory(userID)))
	}
}
