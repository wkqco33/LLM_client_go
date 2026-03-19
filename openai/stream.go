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
	done    chan struct{}
	closed  bool
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

	st := &Stream{
		resp:    resp,
		scanner: bufio.NewScanner(resp.Body),
		done:    make(chan struct{}),
	}
	go func() {
		select {
		case <-ctx.Done():
			st.Close()
		case <-st.done:
		}
	}()
	return st, nil
}

// Next reads the next chunk from the stream.
// Returns (nil, io.EOF) when the stream is complete.
// Returns (nil, llm.ErrStreamClosed) if the stream has already been closed.
func (s *Stream) Next() (*llm.ChatStreamChunk, error) {
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

		var chunk llm.ChatStreamChunk
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
	close(s.done)
	return s.resp.Body.Close()
}
