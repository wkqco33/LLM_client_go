package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	llm "llm-client-go"
	"llm-client-go/internal/sse"
)

// Stream reads a server-sent event (SSE) stream from the Chat Completions API.
type Stream struct {
	conn *sse.Conn
}

// Stream starts a streaming chat completion request.
// The caller must call Close() on the returned Stream when done.
func (s *ChatService) Stream(ctx context.Context, req llm.ChatRequest) (llm.Stream, error) {
	req.Stream = true

	resp, err := s.client.do(ctx, http.MethodPost, "/chat/completions", req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	return &Stream{conn: sse.New(ctx, resp)}, nil
}

// Next reads the next chunk from the stream.
// Returns (nil, nil) when the stream is complete.
// Returns (nil, llm.ErrStreamClosed) if the stream has already been closed.
func (s *Stream) Next() (*llm.ChatStreamChunk, error) {
	if s.conn.Closed() {
		return nil, llm.ErrStreamClosed
	}

	data, ok, err := s.conn.Next()
	if err != nil {
		return nil, fmt.Errorf("openai: read stream: %w", err)
	}
	if !ok || data == "[DONE]" {
		return nil, nil
	}

	var chunk llm.ChatStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, fmt.Errorf("openai: parse stream chunk: %w", err)
	}
	return &chunk, nil
}

// Close releases resources held by the stream.
func (s *Stream) Close() error {
	return s.conn.Close()
}
