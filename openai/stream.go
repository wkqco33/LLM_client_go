package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	llm "llm-client-go"
)

// Stream reads a server-sent event (SSE) stream from the Chat Completions API.
type Stream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	closed  bool
}

// ChatStreamChunk is a single SSE delta from the streaming response.
type ChatStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *llm.Usage     `json:"usage,omitempty"` // only present in the final chunk when IncludeUsage is set
}

// StreamChoice is a single delta in a streaming chunk.
type StreamChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

// Delta carries the incremental content in a stream chunk.
type Delta struct {
	Role      llm.Role        `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`
}

// ToolCallDelta carries the incremental tool call data in a stream chunk.
type ToolCallDelta struct {
	Index    int               `json:"index"`
	ID       string            `json:"id,omitempty"`
	Type     string            `json:"type,omitempty"`
	Function FunctionCallDelta `json:"function,omitempty"`
}

// FunctionCallDelta carries partial function call information.
type FunctionCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// Stream starts a streaming chat completion request.
// The caller must call Close() on the returned Stream when done.
func (s *ChatService) Stream(ctx context.Context, req ChatRequest) (*Stream, error) {
	req.Stream = true

	resp, err := s.client.do(ctx, http.MethodPost, "/chat/completions", req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	return &Stream{
		resp:    resp,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

// Next reads the next chunk from the stream.
// Returns (nil, io.EOF) when the stream is complete.
// Returns (nil, llm.ErrStreamClosed) if the stream has already been closed.
func (s *Stream) Next() (*ChatStreamChunk, error) {
	if s.closed {
		return nil, llm.ErrStreamClosed
	}

	for s.scanner.Scan() {
		line := s.scanner.Text()

		// SSE lines start with "data: "
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// The stream ends with the sentinel value.
		if data == "[DONE]" {
			return nil, nil
		}

		var chunk ChatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, fmt.Errorf("openai: parse stream chunk: %w", err)
		}
		return &chunk, nil
	}

	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("openai: read stream: %w", err)
	}

	return nil, nil
}

// Close releases resources held by the stream.
func (s *Stream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.resp.Body.Close()
}
