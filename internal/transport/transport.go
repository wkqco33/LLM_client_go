// Package transport provides the shared HTTP client construction and
// request-execution boilerplate used by every provider's client.go: resolving
// the *http.Client (custom client / timeout / retry policy), and marshaling +
// sending a JSON request with provider-specific headers.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/wkqco33/LLM_client_go/retry"
)

// BuildHTTPClient resolves the *http.Client to use: custom if provided, otherwise a client
// configured with the given timeout and retry policy.
func BuildHTTPClient(custom *http.Client, timeout time.Duration, policy *retry.Policy) *http.Client {
	hc := custom
	if hc == nil {
		hc = &http.Client{Timeout: timeout}
	}
	p := retry.DefaultPolicy
	if policy != nil {
		p = *policy
	}
	ApplyRetryPolicy(hc, p)
	return hc
}

// ApplyRetryPolicy wraps hc's Transport with a retrying RoundTripper following policy.
func ApplyRetryPolicy(hc *http.Client, policy retry.Policy) {
	hc.Transport = retry.NewRoundTripper(hc.Transport, policy)
}

// DecodeJSON decodes a successful JSON response into type T, or returns parseErr on non-200.
func DecodeJSON[T any](provider string, resp *http.Response, parseErr func(*http.Response) error) (*T, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, parseErr(resp)
	}
	defer resp.Body.Close()

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", provider, err)
	}
	return &result, nil
}

// Do marshals body as JSON, sets custom headers, and executes the HTTP request on hc.
func Do(ctx context.Context, hc *http.Client, provider, method, url string, body any, setHeaders func(*http.Request)) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%s: marshal request: %w", provider, err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", provider, err)
	}
	if setHeaders != nil {
		setHeaders(req)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: http request: %w", provider, err)
	}
	return resp, nil
}
