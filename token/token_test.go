package token

import (
	"testing"

	llm "github.com/wkqco33/LLM_client_go"
)

func TestHeuristicCounter_Count_EmptyString_ReturnsZero(t *testing.T) {
	c := HeuristicCounter{}
	if got := c.Count(""); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestHeuristicCounter_Count_ShortString_ReturnsMinimumOne(t *testing.T) {
	c := HeuristicCounter{}
	if got := c.Count("a"); got < 1 {
		t.Errorf("got %d, want >= 1", got)
	}
}

func TestHeuristicCounter_Count_TableDriven(t *testing.T) {
	c := HeuristicCounter{}
	tests := []struct {
		name string
		text string
		min  int
		max  int
	}{
		{
			name: "short english sentence",
			text: "Hello, world!",
			min:  3,
			max:  6,
		},
		{
			name: "korean text",
			text: "안녕하세요. 반갑습니다.",
			min:  3,
			max:  7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Count(tt.text)
			if got < tt.min || got > tt.max {
				t.Errorf("Count(%q) = %d; want between [%d, %d]", tt.text, got, tt.min, tt.max)
			}
		})
	}
}

func TestHeuristicCounter_CountMessages_IncludesOverhead(t *testing.T) {
	c := HeuristicCounter{}
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helper."},
		{Role: llm.RoleUser, Content: "Hi there."},
	}

	got := c.CountMessages(messages)
	// Base overhead: 4 per message (8) + 3 (assistant prefix) = 11 + content tokens
	if got < 15 {
		t.Errorf("CountMessages() = %d; expected at least 15", got)
	}
}

func TestHeuristicCounter_CountMessages_WithToolCalls(t *testing.T) {
	c := HeuristicCounter{}
	messages := []llm.Message{
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"Seoul"}`,
					},
				},
			},
		},
	}

	got := c.CountMessages(messages)
	// Base overhead: 4 + 3 + 10 (tool call) + content of tool name & args
	if got < 20 {
		t.Errorf("CountMessages() with tool calls = %d; expected at least 20", got)
	}
}

func TestEstimate_MatchesDefaultCounter(t *testing.T) {
	text := "Sample test text for token estimation."
	if got, want := Estimate(text), DefaultCounter.Count(text); got != want {
		t.Errorf("Estimate(%q) = %d, want %d", text, got, want)
	}
}
