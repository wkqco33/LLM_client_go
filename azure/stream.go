package azure

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	llm "llm-client-go"
)

// Stream reads a server-sent event (SSE) stream from the Azure OpenAI Chat Completions API.
type Stream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	done    chan struct{}
	closed  bool
}

// Stream starts a streaming chat completion request to Azure OpenAI.
// The caller must call Close() on the returned Stream when done.
func (s *ChatService) Stream(ctx context.Context, req llm.ChatRequest) (llm.Stream, error) {
	req.Stream = true
	// In Azure, the "Model" in the common request is used as the DeploymentName.
	url := s.client.deploymentURL(req.Model, "/chat/completions")

	resp, err := s.client.do(ctx, http.MethodPost, url, req)
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
// Returns (nil, nil) when the stream ends with [DONE].
// Returns (nil, llm.ErrStreamClosed) if the stream has already been closed.
func (s *Stream) Next() (*llm.ChatStreamChunk, error) {
	if s.closed {
		return nil, llm.ErrStreamClosed
	}

	for s.scanner.Scan() {
		line := s.scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return nil, nil
		}

		var chunk llm.ChatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, fmt.Errorf("azure: parse stream chunk: %w", err)
		}
		return &chunk, nil
	}

	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("azure: read stream: %w", err)
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
