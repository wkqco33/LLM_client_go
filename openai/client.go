// Package openai provides a client for the OpenAI Chat Completions API,
// including streaming and function calling support.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.openai.com/v1"
const defaultTimeout = 60 * time.Second

// Client is the OpenAI API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client

	// Chat exposes the Chat Completions API.
	Chat *ChatService
}

// Config holds configuration for the OpenAI client.
type Config struct {
	// APIKey is the OpenAI API key (required).
	APIKey string

	// BaseURL overrides the default API endpoint.
	// Defaults to https://api.openai.com/v1
	BaseURL string

	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client

	// Timeout sets the HTTP request timeout.
	// Defaults to 60 seconds.
	Timeout time.Duration
}

// Option is a functional option for configuring the client after construction.
type Option func(*Client)

// WithBaseURL sets a custom base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// New creates a new OpenAI Client from the provided Config.
// Additional Options can override Config values after initialization.
func New(cfg Config, opts ...Option) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: timeout}
	}

	c := &Client{
		apiKey:     cfg.APIKey,
		baseURL:    baseURL,
		httpClient: hc,
	}

	for _, o := range opts {
		o(c)
	}

	c.Chat = &ChatService{client: c}
	return c
}

// do executes an HTTP request with OpenAI auth headers and returns the response body.
// Callers are responsible for closing the body when streaming is not needed.
func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("openai: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: http request: %w", err)
	}

	return resp, nil
}
