// Package gemini provides a client for the Google Gemini API (v1beta).
// The Gemini API uses a different request/response format than OpenAI,
// so this package handles the conversion to/from the common llm types.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	llm "llm-client-go"
	"llm-client-go/retry"
	"llm-client-go/token"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
const defaultTimeout = 60 * time.Second

// Client is the Google Gemini API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client

	// Chat exposes the generateContent API.
	Chat *ChatService
}

// Complete implements the llm.Client interface.
func (c *Client) Complete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return c.Chat.Complete(ctx, req)
}

// Stream implements the llm.Client interface.
func (c *Client) Stream(ctx context.Context, req llm.ChatRequest) (llm.Stream, error) {
	return c.Chat.Stream(ctx, req)
}

// CreateEmbeddings implements the llm.Client interface.
// Note: Gemini embedding models use a separate endpoint not covered here.
func (c *Client) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, fmt.Errorf("gemini: use the embedContent endpoint for embeddings (not supported by this client)")
}

// TokenCounter implements the llm.Client interface.
func (c *Client) TokenCounter(model string) any {
	return token.HeuristicCounter{}
}

// Config holds configuration for the Gemini client.
type Config struct {
	// APIKey is the Google AI API key (required).
	APIKey string

	// BaseURL overrides the default API endpoint.
	// Defaults to https://generativelanguage.googleapis.com/v1beta
	BaseURL string

	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client

	// Timeout sets the HTTP request timeout.
	// Defaults to 60 seconds.
	Timeout time.Duration

	// RetryPolicy configures automatic retries for failed requests.
	RetryPolicy *retry.Policy
}

// Option is a functional option for configuring the client.
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

// WithRetryPolicy sets the retry policy for the client.
func WithRetryPolicy(p retry.Policy) Option {
	return func(c *Client) {
		c.httpClient.Transport = retry.NewRoundTripper(c.httpClient.Transport, p)
	}
}

// New creates a new Gemini Client from the provided Config.
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

	if cfg.RetryPolicy != nil {
		hc.Transport = retry.NewRoundTripper(hc.Transport, *cfg.RetryPolicy)
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

// modelURL builds the full URL for a model action.
// e.g., https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro:generateContent?key=...
func (c *Client) modelURL(model, action string) string {
	return fmt.Sprintf("%s/models/%s:%s?key=%s", c.baseURL, model, action, c.apiKey)
}

// do executes an HTTP POST request to a Gemini endpoint.
func (c *Client) do(ctx context.Context, url string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: http request: %w", err)
	}

	return resp, nil
}

// doGet executes an HTTP GET request (used for future endpoints).
func (c *Client) doGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: http request: %w", err)
	}

	return resp, nil
}

// readBody is a helper to read and close a response body.
func readBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
