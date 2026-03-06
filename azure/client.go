// Package azure provides a client for the Azure OpenAI Service.
// Azure OpenAI uses the same REST API as OpenAI but with deployment-based URLs,
// api-key header authentication, and a required api-version query parameter.
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultAPIVersion = "2024-02-01"
const defaultTimeout = 60 * time.Second

// Client is the Azure OpenAI API client.
type Client struct {
	endpoint   string // e.g., https://my-resource.openai.azure.com
	apiKey     string
	apiVersion string
	httpClient *http.Client

	// Chat exposes the Chat Completions API.
	Chat *ChatService
}

// Config holds configuration for the Azure OpenAI client.
type Config struct {
	// Endpoint is the Azure OpenAI resource endpoint (required).
	// Format: https://{resource-name}.openai.azure.com
	Endpoint string

	// APIKey is the Azure OpenAI API key (required).
	APIKey string

	// APIVersion is the REST API version to use.
	// Defaults to "2024-02-01".
	APIVersion string

	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client

	// Timeout sets the HTTP request timeout.
	// Defaults to 60 seconds.
	Timeout time.Duration
}

// Option is a functional option for configuring the Azure client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// WithAPIVersion overrides the Azure OpenAI REST API version.
func WithAPIVersion(version string) Option {
	return func(c *Client) { c.apiVersion = version }
}

// New creates a new Azure OpenAI Client from the provided Config.
func New(cfg Config, opts ...Option) *Client {
	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = defaultAPIVersion
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: timeout}
	}

	endpoint := strings.TrimRight(cfg.Endpoint, "/")

	c := &Client{
		endpoint:   endpoint,
		apiKey:     cfg.APIKey,
		apiVersion: apiVersion,
		httpClient: hc,
	}

	for _, o := range opts {
		o(c)
	}

	c.Chat = &ChatService{client: c}
	return c
}

// deploymentURL builds the full URL for a given deployment and path segment.
// e.g., https://resource.openai.azure.com/openai/deployments/gpt4/chat/completions?api-version=2024-02-01
func (c *Client) deploymentURL(deploymentName, path string) string {
	return fmt.Sprintf(
		"%s/openai/deployments/%s%s?api-version=%s",
		c.endpoint, deploymentName, path, c.apiVersion,
	)
}

// do executes an HTTP request with Azure api-key authentication.
func (c *Client) do(ctx context.Context, method, url string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("azure: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("azure: create request: %w", err)
	}

	// Azure uses "api-key" header instead of "Authorization: Bearer"
	req.Header.Set("api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: http request: %w", err)
	}

	return resp, nil
}
