// Package openai provides a client for the OpenAI Chat Completions API,
// including streaming and function calling support.
package openai

import (
	"context"
	"net/http"
	"time"

	llm "github.com/wkqco33/LLM_client_go"
	"github.com/wkqco33/LLM_client_go/internal/transport"
	"github.com/wkqco33/LLM_client_go/retry"
	"github.com/wkqco33/LLM_client_go/token"
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

// Complete implements the llm.Client interface.
func (c *Client) Complete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return c.Chat.Complete(ctx, req)
}

// Stream implements the llm.Client interface.
func (c *Client) Stream(ctx context.Context, req llm.ChatRequest) (llm.Stream, error) {
	return c.Chat.Stream(ctx, req)
}

// CreateEmbeddings implements the llm.Client interface.
func (c *Client) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	resp, err := c.do(ctx, http.MethodPost, "/embeddings", req)
	if err != nil {
		return nil, err
	}
	return transport.DecodeJSON[llm.EmbeddingResponse]("openai", resp, parseErrorResponse)
}

// TokenCounter implements the llm.Client interface.
func (c *Client) TokenCounter(model string) any {
	return token.HeuristicCounter{}
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

	// RetryPolicy configures automatic retries for failed requests.
	// Defaults to retry.DefaultPolicy; pass &retry.Policy{} to disable retries.
	RetryPolicy *retry.Policy
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

// WithRetryPolicy sets the retry policy for the client.
func WithRetryPolicy(p retry.Policy) Option {
	return func(c *Client) { transport.ApplyRetryPolicy(c.httpClient, p) }
}

// New creates a new OpenAI Client from the provided Config.
func New(cfg Config, opts ...Option) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	c := &Client{
		apiKey:     cfg.APIKey,
		baseURL:    baseURL,
		httpClient: transport.BuildHTTPClient(cfg.HTTPClient, timeout, cfg.RetryPolicy),
	}

	for _, o := range opts {
		o(c)
	}

	c.Chat = &ChatService{client: c}
	return c
}

// do executes an HTTP request with OpenAI auth headers.
func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	return transport.Do(ctx, c.httpClient, "openai", method, c.baseURL+path, body, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
	})
}
