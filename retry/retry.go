package retry

import (
	"bytes"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// Policy defines the retry strategy.
type Policy struct {
	MaxRetries    int
	MinWait       time.Duration
	MaxWait       time.Duration
	RetryOnStatus []int
}

// DefaultPolicy provides a sensible retry configuration.
var DefaultPolicy = Policy{
	MaxRetries: 3,
	MinWait:    1 * time.Second,
	MaxWait:    30 * time.Second,
	RetryOnStatus: []int{
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout,      // 504
	},
}

// RoundTripper implements http.RoundTripper with retry logic.
type RoundTripper struct {
	Base   http.RoundTripper
	Policy Policy
}

// NewRoundTripper wraps an existing RoundTripper with the provided Policy.
func NewRoundTripper(base http.RoundTripper, policy Policy) *RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RoundTripper{
		Base:   base,
		Policy: policy,
	}
}

// RoundTrip executes a request with retries.
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
		body []byte
	)

	// If there's a body, we need to read it into memory because it can only be read once.
	if req.Body != nil && req.Body != http.NoBody {
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; attempt <= rt.Policy.MaxRetries; attempt++ {
		// Reset the request body for each retry
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}

		// Perform the actual HTTP request
		resp, err = rt.Base.RoundTrip(req)

		// Check if we should retry
		shouldRetry := false

		if err != nil {
			// Network errors, timeouts, etc.
			shouldRetry = true
		} else if rt.isRetryableStatus(resp.StatusCode) {
			shouldRetry = true
		}

		// If no retry is needed or we've reached the limit, return the result
		if !shouldRetry || attempt == rt.Policy.MaxRetries {
			return resp, err
		}

		// Prepare for the next retry
		if resp != nil {
			// Must drain and close the body of the failed response
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		// Calculate backoff time
		wait := rt.calculateBackoff(attempt, resp)
		
		select {
		case <-time.After(wait):
			// Proceed to the next attempt
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	return resp, err
}

func (rt *RoundTripper) isRetryableStatus(code int) bool {
	for _, status := range rt.Policy.RetryOnStatus {
		if code == status {
			return true
		}
	}
	return false
}

func (rt *RoundTripper) calculateBackoff(attempt int, resp *http.Response) time.Duration {
	// Respect Retry-After header if present
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if seconds, err := strconv.Atoi(ra); err == nil {
				return time.Duration(seconds) * time.Second
			}
			// It could also be a date string, but for brevity we'll stick to seconds
		}
	}

	// Exponential backoff with jitter
	// wait = min(MaxWait, MinWait * 2^attempt)
	exp := math.Pow(2, float64(attempt))
	wait := time.Duration(float64(rt.Policy.MinWait) * exp)
	
	if wait > rt.Policy.MaxWait {
		wait = rt.Policy.MaxWait
	}

	// Add jitter (±20%)
	jitter := float64(wait) * 0.2
	wait += time.Duration(rand.Float64()*2*jitter - jitter)

	return wait
}
