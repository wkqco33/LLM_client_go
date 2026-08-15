package token

import (
	"unicode/utf8"

	llm "github.com/wkqco33/LLM_client_go"
)

// Counter defines the interface for token counting.
type Counter interface {
	// Count returns the estimated number of tokens in a string.
	Count(text string) int

	// CountMessages returns the estimated number of tokens in a slice of messages.
	// This accounts for role names and other message overhead.
	CountMessages(messages []llm.Message) int
}

// ─── Heuristic Estimator (Default Fallback) ──────────────────

// HeuristicCounter provides a rough estimate of token counts without external dependencies.
// It uses common heuristics: ~4 characters per token for English, more for CJK.
type HeuristicCounter struct{}

func (h HeuristicCounter) Count(text string) int {
	charCount := utf8.RuneCountInString(text)
	if charCount == 0 {
		return 0
	}
	// Rough heuristic: 1 token ≈ 4 chars for English.
	// We'll use a slightly more conservative 3.5 to avoid underestimation.
	tokens := float64(charCount) / 3.5
	if tokens < 1 {
		return 1
	}
	return int(tokens)
}

func (h HeuristicCounter) CountMessages(messages []llm.Message) int {
	total := 0
	for _, m := range messages {
		total += 4 // overhead for role and metadata
		total += h.Count(m.Content)
		for _, tc := range m.ToolCalls {
			total += 10 // overhead for tool call
			total += h.Count(tc.Function.Name)
			total += h.Count(tc.Function.Arguments)
		}
	}
	total += 3 // final assistant reply overhead
	return total
}

// ─── Global Helper ───────────────────────────────────────────

// DefaultCounter is a shared instance of the heuristic counter.
var DefaultCounter = HeuristicCounter{}

// Estimate returns a rough token count for any text using the default heuristic.
func Estimate(text string) int {
	return DefaultCounter.Count(text)
}
