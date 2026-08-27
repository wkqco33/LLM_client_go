// Package sse provides a Server-Sent Events (SSE) reader for streaming HTTP responses.
package sse

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"sync"
)

// maxLineSize bounds a single SSE line (1MB).
const maxLineSize = 1 << 20

// Conn scans SSE "data: " lines from an HTTP response body.
type Conn struct {
	resp    *http.Response
	scanner *bufio.Scanner
	done    chan struct{}
	mu      sync.Mutex
	closed  bool
}

// New returns a Conn that scans SSE data lines from resp.Body and closes on ctx cancellation or Close.
func New(ctx context.Context, resp *http.Response) *Conn {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	c := &Conn{
		resp:    resp,
		scanner: scanner,
		done:    make(chan struct{}),
	}
	go func() {
		select {
		case <-ctx.Done():
			c.Close()
		case <-c.done:
		}
	}()
	return c
}

// Next returns the next SSE "data: " payload with the prefix stripped,
// skipping any other lines (blank lines, comments, other fields).
// It returns ok=false and a nil error at normal end of stream.
func (c *Conn) Next() (data string, ok bool, err error) {
	for c.scanner.Scan() {
		line := c.scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		return strings.TrimPrefix(line, "data: "), true, nil
	}
	if err := c.scanner.Err(); err != nil {
		return "", false, err
	}
	return "", false, nil
}

// Closed reports whether Close has already been called.
func (c *Conn) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// Close releases the underlying response body. Safe to call multiple times.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.done)
	return c.resp.Body.Close()
}
