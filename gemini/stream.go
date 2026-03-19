package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	llm "llm-client-go"
)

// Stream handles the Gemini SSE stream.
type Stream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	done    chan struct{}
	closed  bool
}

// Stream starts a streaming chat completion request via streamGenerateContent.
func (s *ChatService) Stream(ctx context.Context, req llm.ChatRequest) (llm.Stream, error) {
	gReq, err := s.toGeminiRequest(req)
	if err != nil {
		return nil, err
	}

	url := s.client.modelURL(req.Model, "streamGenerateContent")
	resp, err := s.client.do(ctx, url, gReq)
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

// Next reads the next chunk from the Gemini SSE stream.
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

		var event generateContentResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if len(event.Candidates) == 0 {
			continue
		}

		chunk := &llm.ChatStreamChunk{}

		if event.UsageMetadata.TotalTokenCount > 0 {
			chunk.Usage = &llm.Usage{
				PromptTokens:     event.UsageMetadata.PromptTokenCount,
				CompletionTokens: event.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      event.UsageMetadata.TotalTokenCount,
			}
		}

		for i, cand := range event.Candidates {
			var deltaContent string
			for _, p := range cand.Content.Parts {
				deltaContent += p.Text
			}

			choice := llm.StreamChoice{
				Index: i,
				Delta: llm.Delta{Content: deltaContent},
			}

			if cand.FinishReason != "" {
				reason := mapFinishReason(cand.FinishReason)
				choice.FinishReason = &reason
			}

			chunk.Choices = append(chunk.Choices, choice)
		}

		return chunk, nil
	}

	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("gemini: read stream: %w", err)
	}

	return nil, nil
}

// Close releases the HTTP response body.
func (s *Stream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	return s.resp.Body.Close()
}
