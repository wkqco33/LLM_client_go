package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	llm "llm-client-go"
)

// Stream handles the Anthropic SSE (Server-Sent Events) stream.
type Stream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	done    chan struct{}
	closed  bool
}

// anthropicEvent represents a single event in the Anthropic stream.
type anthropicEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta *anthropicDelta `json:"delta,omitempty"`
	Message *messageResponse `json:"message,omitempty"` // for message_start
	Usage   *usage           `json:"usage,omitempty"`   // for message_delta
}

type anthropicDelta struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	PartialJSON  string        `json:"partial_json,omitempty"` // for tool_use
	StopReason   string        `json:"stop_reason,omitempty"`
	StopSequence string        `json:"stop_sequence,omitempty"`
}

// Stream starts a streaming chat completion request.
func (s *ChatService) Stream(ctx context.Context, req llm.ChatRequest) (llm.Stream, error) {
	antReq, err := s.toAnthropicRequest(req)
	if err != nil {
		return nil, err
	}
	antReq.Stream = true

	resp, err := s.client.do(ctx, http.MethodPost, "/messages", antReq)
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

// Next reads the next chunk and maps it to the common llm.ChatStreamChunk.
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
		var event anthropicEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		chunk := &llm.ChatStreamChunk{}

		switch event.Type {
		case "message_start":
			chunk.ID = event.Message.ID
			chunk.Model = event.Message.Model
			chunk.Choices = []llm.StreamChoice{{
				Index: 0,
				Delta: llm.Delta{Role: llm.RoleAssistant},
			}}
			return chunk, nil

		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			chunk.Choices = []llm.StreamChoice{{
				Index: event.Index,
				Delta: llm.Delta{Content: event.Delta.Text},
			}}
			// Tool use delta handling can be added here if needed
			return chunk, nil

		case "message_delta":
			if event.Usage != nil {
				chunk.Usage = &llm.Usage{
					PromptTokens:     event.Usage.InputTokens,
					CompletionTokens: event.Usage.OutputTokens,
					TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
				}
			}
			if event.Delta != nil && event.Delta.StopReason != "" {
				reason := event.Delta.StopReason
				if reason == "end_turn" {
					reason = "stop"
				}
				chunk.Choices = []llm.StreamChoice{{
					Index:        0,
					FinishReason: &reason,
				}}
			}
			return chunk, nil

		case "message_stop":
			return nil, nil // End of stream

		case "error":
			return nil, fmt.Errorf("anthropic stream error: %s", data)
		}
	}

	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("anthropic: read stream: %w", err)
	}

	return nil, nil
}

func (s *Stream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	return s.resp.Body.Close()
}
