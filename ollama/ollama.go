// Package ollama provides a client for local Ollama inference servers.
// Ollama exposes an OpenAI-compatible REST API, so this package is a thin
// wrapper over the openai package with Ollama-specific defaults.
package ollama

import (
	"net/http"
	"time"

	"github.com/wkqco33/LLM_client_go/openai"
	"github.com/wkqco33/LLM_client_go/retry"
)

const defaultBaseURL = "http://localhost:11434/v1"

// Config holds configuration for the Ollama client.
type Config struct {
	// BaseURL overrides the default Ollama endpoint.
	// Defaults to http://localhost:11434/v1
	BaseURL string

	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client

	// Timeout sets the HTTP request timeout.
	// Defaults to 60 seconds. Large local models may need longer timeouts.
	Timeout time.Duration

	// RetryPolicy configures automatic retries for failed requests.
	// Defaults to retry.DefaultPolicy; pass &retry.Policy{} to disable retries.
	RetryPolicy *retry.Policy
}

// New creates an OpenAI-compatible client pointed at a local Ollama instance.
// The returned *openai.Client supports all the same methods (Complete, Stream, etc.).
func New(cfg Config, opts ...openai.Option) *openai.Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return openai.New(openai.Config{
		APIKey:      "ollama", // Ollama doesn't require a real API key
		BaseURL:     baseURL,
		HTTPClient:  cfg.HTTPClient,
		Timeout:     cfg.Timeout,
		RetryPolicy: cfg.RetryPolicy,
	}, opts...)
}
