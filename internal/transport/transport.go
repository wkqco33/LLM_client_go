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

	"llm-client-go/retry"
)

// BuildHTTPClient resolves the *http.Client a provider should use: custom if
// non-nil, otherwise a fresh client with the given timeout. The resulting
// client's Transport is always wrapped with a retrying RoundTripper —
// retry.DefaultPolicy unless policy overrides it, so a zero-config client
// still retries transient 429/5xx failures instead of surfacing them
// straight to the caller. To disable retries entirely, pass an explicit
// &retry.Policy{} (MaxRetries: 0).
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

// ApplyRetryPolicy wraps hc's Transport with a retrying RoundTripper
// following policy.
func ApplyRetryPolicy(hc *http.Client, policy retry.Policy) {
	hc.Transport = retry.NewRoundTripper(hc.Transport, policy)
}

// DecodeJSON checks resp's status code: on http.StatusOK it decodes the
// body as JSON into a new T and closes the body; on any other status it
// delegates to parseErr(resp), which owns closing the body itself (every
// provider's parseErrorResponse already does this, since it's also called
// directly from streaming code paths that never reach DecodeJSON).
//
// This is the "do request, check status, decode JSON" tail that every
// provider's Complete/CreateEmbeddings shared before being factored out
// here.
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

// Do marshals body (if non-nil) as JSON, builds a request for method/url,
// invokes setHeaders to attach provider-specific auth/content headers, and
// executes it on hc. Errors are wrapped with provider for context, matching
// the "<provider>: <stage>: %w" convention every client used before this was
// shared.
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
