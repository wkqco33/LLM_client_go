package bots

import (
	"sync"
	"time"

	llm "github.com/wkqco33/LLM_client_go"
)

const defaultMaxHistory = 20

// SessionManager maintains per-user conversation histories in memory.
// It is safe for concurrent use. Call Close when done with it to stop the
// background TTL cleanup goroutine (see WithTTL) — otherwise an application
// that creates and discards many SessionManagers will leak one goroutine per
// instance.
type SessionManager struct {
	mu         sync.RWMutex
	sessions   map[string][]llm.Message
	lastSeen   map[string]time.Time
	maxHistory int
	ttl        time.Duration
	systemMsg  *llm.Message

	done      chan struct{}
	closeOnce sync.Once
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

// WithTTL sets the time-to-live for inactive sessions.
// Sessions not accessed within the TTL are automatically removed.
// 0 means no expiration (default).
func WithTTL(d time.Duration) SessionOption {
	return func(sm *SessionManager) { sm.ttl = d }
}

// NewSessionManager creates a new SessionManager with the given options.
func NewSessionManager(opts ...SessionOption) *SessionManager {
	sm := &SessionManager{
		sessions:   make(map[string][]llm.Message),
		lastSeen:   make(map[string]time.Time),
		maxHistory: defaultMaxHistory,
		done:       make(chan struct{}),
	}
	for _, o := range opts {
		o(sm)
	}
	if sm.ttl > 0 {
		go sm.cleanupLoop()
	}
	return sm
}

// Close stops the background TTL cleanup goroutine started by WithTTL. It's
// a no-op if TTL wasn't configured, and safe to call multiple times or
// concurrently.
func (sm *SessionManager) Close() {
	sm.closeOnce.Do(func() {
		close(sm.done)
	})
}

func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(sm.ttl / 2)
	defer ticker.Stop()
	for {
		select {
		case <-sm.done:
			return
		case <-ticker.C:
			now := time.Now()
			sm.mu.Lock()
			for id, t := range sm.lastSeen {
				if now.Sub(t) > sm.ttl {
					delete(sm.sessions, id)
					delete(sm.lastSeen, id)
				}
			}
			sm.mu.Unlock()
		}
	}
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

	if sm.ttl > 0 {
		sm.lastSeen[userID] = time.Now()
	}

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
	delete(sm.lastSeen, userID)
}

// SplitMessage splits text into chunks of at most maxLen characters, for
// platforms with a per-message length limit (Discord: 2000, Telegram: 4096,
// Slack: no hard API limit but very long messages should still be chunked).
// maxLen <= 0 disables chunking.
func SplitMessage(text string, maxLen int) []string {
	if maxLen <= 0 || len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > maxLen {
		chunks = append(chunks, text[:maxLen])
		text = text[maxLen:]
	}
	if text != "" {
		chunks = append(chunks, text)
	}
	return chunks
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
