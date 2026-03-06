package bots

import (
	"sync"

	llm "llm-client-go"
)

const defaultMaxHistory = 20

// SessionManager maintains per-user conversation histories in memory.
// It is safe for concurrent use.
type SessionManager struct {
	mu         sync.RWMutex
	sessions   map[string][]llm.Message
	maxHistory int
	systemMsg  *llm.Message
}

// SessionOption configures a SessionManager.
type SessionOption func(*SessionManager)

// WithMaxHistory sets the maximum number of messages kept per user.
// When the limit is reached, the oldest non-system messages are removed.
func WithMaxHistory(n int) SessionOption {
	return func(sm *SessionManager) { sm.maxHistory = n }
}

// WithSystemPrompt prepends a fixed system message to every user's conversation.
func WithSystemPrompt(content string) SessionOption {
	return func(sm *SessionManager) {
		msg := llm.Message{Role: llm.RoleSystem, Content: content}
		sm.systemMsg = &msg
	}
}

// NewSessionManager creates a new SessionManager with the given options.
func NewSessionManager(opts ...SessionOption) *SessionManager {
	sm := &SessionManager{
		sessions:   make(map[string][]llm.Message),
		maxHistory: defaultMaxHistory,
	}
	for _, o := range opts {
		o(sm)
	}
	return sm
}

// GetHistory returns a copy of the conversation history for the given user ID.
func (sm *SessionManager) GetHistory(userID string) []llm.Message {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	msgs := sm.sessions[userID]
	if len(msgs) == 0 && sm.systemMsg != nil {
		return []llm.Message{*sm.systemMsg}
	}
	out := make([]llm.Message, len(msgs))
	copy(out, msgs)
	return out
}

// Append adds a message to the user's history, trimming old messages if needed.
func (sm *SessionManager) Append(userID string, msg llm.Message) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	history := sm.sessions[userID]

	// Initialize with system message if configured and history is empty.
	if len(history) == 0 && sm.systemMsg != nil {
		history = []llm.Message{*sm.systemMsg}
	}

	history = append(history, msg)
	history = sm.trim(history)
	sm.sessions[userID] = history
}

// Reset clears the conversation history for the given user ID.
func (sm *SessionManager) Reset(userID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, userID)
}

// trim removes the oldest non-system messages when history exceeds maxHistory.
func (sm *SessionManager) trim(history []llm.Message) []llm.Message {
	if sm.maxHistory <= 0 || len(history) <= sm.maxHistory {
		return history
	}

	// Always preserve the system message at index 0 if present.
	start := 0
	if len(history) > 0 && history[0].Role == llm.RoleSystem {
		start = 1
	}

	excess := len(history) - sm.maxHistory
	return append(history[:start], history[start+excess:]...)
}
