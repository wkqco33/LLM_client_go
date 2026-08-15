package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"

	llm "github.com/wkqco33/LLM_client_go"
)

// ErrMaxTurnsExceeded is returned by Run when the tool-calling loop reaches
// its configured turn limit (see WithMaxTurns) without the model producing a
// final, non-tool-call response.
var ErrMaxTurnsExceeded = errors.New("agent: max turns exceeded")

// ExecutableTool represents a tool that can be executed locally.
// It provides both the schema definition for the LLM and the execution logic.
type ExecutableTool interface {
	// Definition returns the tool schema for the LLM.
	Definition() llm.Tool

	// Execute runs the local tool logic using the JSON arguments provided by the LLM.
	// It should return a string representation of the result (often JSON) to send back.
	Execute(ctx context.Context, arguments string) (string, error)
}

// Runner orchestrates the conversation loop between the user, the LLM, and the tools.
type Runner struct {
	client     llm.Client
	model      string
	tools      map[string]ExecutableTool
	maxTurn    int
	maxHistory int
	systemMsg  *llm.Message
}

// RunnerOption is a functional option for configuring a Runner.
type RunnerOption func(*Runner)

// WithMaxTurns sets the maximum number of tool-call iterations before giving up.
// Defaults to 5.
func WithMaxTurns(turns int) RunnerOption {
	return func(r *Runner) { r.maxTurn = turns }
}

// WithMaxHistory sets the maximum number of messages to keep in context.
// Older messages (excluding system prompt) will be trimmed. 0 means unlimited.
func WithMaxHistory(n int) RunnerOption {
	return func(r *Runner) { r.maxHistory = n }
}

// WithSystemPrompt sets an optional system prompt for the conversation.
func WithSystemPrompt(prompt string) RunnerOption {
	return func(r *Runner) {
		r.systemMsg = &llm.Message{Role: llm.RoleSystem, Content: prompt}
	}
}

// NewRunner creates a new Agent Runner.
func NewRunner(client llm.Client, model string, opts ...RunnerOption) *Runner {
	r := &Runner{
		client:     client,
		model:      model,
		tools:      make(map[string]ExecutableTool),
		maxTurn:    5,
		maxHistory: 0,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// RegisterTool registers an ExecutableTool with the runner.
func (r *Runner) RegisterTool(t ExecutableTool) {
	def := t.Definition()
	r.tools[def.Function.Name] = t
}

// Run executes the conversation loop. It returns the final updated message history
// and the last ChatResponse from the LLM.
func (r *Runner) Run(ctx context.Context, userMessages []llm.Message) ([]llm.Message, *llm.ChatResponse, error) {
	// 1. Prepare messages with trimming
	var messages []llm.Message
	if r.maxHistory > 0 && len(userMessages) > r.maxHistory {
		messages = userMessages[len(userMessages)-r.maxHistory:]
	} else {
		messages = userMessages
	}

	// 2. Prepend system message if set
	fullMessages := make([]llm.Message, 0, len(messages)+1)
	if r.systemMsg != nil {
		fullMessages = append(fullMessages, *r.systemMsg)
	}
	fullMessages = append(fullMessages, messages...)
	messages = fullMessages

	llmTools := make([]llm.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		llmTools = append(llmTools, t.Definition())
	}

	for turn := 0; turn < r.maxTurn; turn++ {
		req := llm.ChatRequest{
			Model:    r.model,
			Messages: messages,
		}
		if len(llmTools) > 0 {
			req.Tools = llmTools
		}

		resp, err := r.client.Complete(ctx, req)
		if err != nil {
			return messages, nil, fmt.Errorf("agent: complete: %w", err)
		}

		if len(resp.Choices) == 0 {
			return messages, resp, fmt.Errorf("agent: empty choices in response")
		}

		assistantMsg := resp.Choices[0].Message
		messages = append(messages, assistantMsg)

		// If the model did not ask to call tools, we are done.
		if resp.Choices[0].FinishReason != "tool_calls" || len(assistantMsg.ToolCalls) == 0 {
			return messages, resp, nil
		}

		// Execute all requested tool calls in parallel — the model can ask
		// for several independent tools in one turn (e.g. weather + search),
		// and there's no reason to pay their latency sequentially. Each
		// goroutine writes only to its own slice index, so results land in
		// results[i] regardless of completion order; they're appended to
		// messages in the original tool_calls order afterward.
		results := make([]llm.Message, len(assistantMsg.ToolCalls))
		var wg sync.WaitGroup
		for i, tc := range assistantMsg.ToolCalls {
			wg.Add(1)
			go func(i int, tc llm.ToolCall) {
				defer wg.Done()
				results[i] = r.executeToolCall(ctx, tc)
			}(i, tc)
		}
		wg.Wait()
		messages = append(messages, results...)
	}

	return messages, nil, fmt.Errorf("%w: reached %d turns without a final response", ErrMaxTurnsExceeded, r.maxTurn)
}

// executeToolCall runs a single tool call and turns the outcome (including a
// missing tool or an execution error) into a RoleTool message. Errors are
// never returned to the caller here — they're surfaced to the model as
// message content so it has a chance to react (retry, apologize, try a
// different tool) instead of aborting the whole run.
func (r *Runner) executeToolCall(ctx context.Context, tc llm.ToolCall) llm.Message {
	tool, ok := r.tools[tc.Function.Name]
	if !ok {
		return llm.Message{
			Role:       llm.RoleTool,
			Content:    fmt.Sprintf("Error: tool %q not found", tc.Function.Name),
			ToolCallID: tc.ID,
		}
	}

	result, err := tool.Execute(ctx, tc.Function.Arguments)
	if err != nil {
		result = fmt.Sprintf("Error executing tool: %v", err)
	}

	return llm.Message{
		Role:       llm.RoleTool,
		Content:    result,
		ToolCallID: tc.ID,
	}
}
