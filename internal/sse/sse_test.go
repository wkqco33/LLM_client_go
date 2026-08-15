package sse

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type trackClosingReader struct {
	io.Reader
	closed int32
}

func (r *trackClosingReader) Close() error {
	atomic.StoreInt32(&r.closed, 1)
	return nil
}

func TestConn_Next_ParsesDataLines(t *testing.T) {
	rawSSE := ": comment\n\ndata: first chunk\n\n: another comment\ndata: second chunk\ndata: [DONE]\n\n"
	body := &trackClosingReader{Reader: strings.NewReader(rawSSE)}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	conn := New(context.Background(), resp)
	defer conn.Close()

	expected := []string{"first chunk", "second chunk", "[DONE]"}
	for _, want := range expected {
		data, ok, err := conn.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatalf("expected ok=true for data %q, got ok=false", want)
		}
		if data != want {
			t.Errorf("got %q, want %q", data, want)
		}
	}

	// End of stream
	_, ok, err := conn.Next()
	if err != nil {
		t.Fatalf("unexpected error at EOF: %v", err)
	}
	if ok {
		t.Errorf("expected ok=false at EOF, got ok=true")
	}
}

func TestConn_Close_MultipleTimes_Safe(t *testing.T) {
	body := &trackClosingReader{Reader: strings.NewReader("data: hi\n\n")}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	conn := New(context.Background(), resp)
	if conn.Closed() {
		t.Errorf("expected not closed initially")
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("first close error: %v", err)
	}
	if !conn.Closed() {
		t.Errorf("expected Closed() to be true")
	}

	// Double close should be a no-op and safe
	if err := conn.Close(); err != nil {
		t.Fatalf("second close error: %v", err)
	}
	if atomic.LoadInt32(&body.closed) != 1 {
		t.Errorf("expected body to be closed exactly once")
	}
}

func TestConn_ContextCancellation_ClosesStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	body := &trackClosingReader{Reader: strings.NewReader("data: stream\n\n")}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}

	conn := New(ctx, resp)
	cancel()

	// Wait briefly for goroutine watching ctx.Done()
	time.Sleep(20 * time.Millisecond)

	if !conn.Closed() {
		t.Errorf("expected conn to be closed after context cancellation")
	}
	if atomic.LoadInt32(&body.closed) != 1 {
		t.Errorf("expected body to be closed by context watcher")
	}
}
