package retry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRoundTrip_Success_NoRetry(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Transport = NewRoundTripper(client.Transport, Policy{
		MaxRetries: 3,
		MinWait:    1 * time.Millisecond,
		MaxWait:    5 * time.Millisecond,
	})

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("got %d calls, want 1", got)
	}
}

func TestRoundTrip_RetryOn500_MaxRetriesExceeded(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	t.Cleanup(srv.Close)

	maxRetries := 2
	client := srv.Client()
	client.Transport = NewRoundTripper(client.Transport, Policy{
		MaxRetries:    maxRetries,
		MinWait:       1 * time.Millisecond,
		MaxWait:       5 * time.Millisecond,
		RetryOnStatus: []int{http.StatusInternalServerError},
	})

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	expectedCalls := int32(maxRetries + 1)
	if got := atomic.LoadInt32(&callCount); got != expectedCalls {
		t.Errorf("got %d calls, want %d", got, expectedCalls)
	}
}

func TestRoundTrip_RetryEventuallySucceeds(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("recovered"))
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Transport = NewRoundTripper(client.Transport, Policy{
		MaxRetries:    3,
		MinWait:       1 * time.Millisecond,
		MaxWait:       5 * time.Millisecond,
		RetryOnStatus: []int{http.StatusServiceUnavailable},
	})

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("got %d calls, want 3", got)
	}
}

func TestRoundTrip_NonRetryableStatus_ReturnsImmediately(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Transport = NewRoundTripper(client.Transport, Policy{
		MaxRetries:    3,
		MinWait:       1 * time.Millisecond,
		MaxWait:       5 * time.Millisecond,
		RetryOnStatus: []int{http.StatusInternalServerError, http.StatusTooManyRequests},
	})

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("got %d calls, want 1", got)
	}
}

func TestRoundTrip_RequestBodyPreserved_OnRetry(t *testing.T) {
	var callCount int32
	var receivedBodies []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		body, _ := io.ReadAll(r.Body)
		receivedBodies = append(receivedBodies, string(body))

		if count == 1 {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Transport = NewRoundTripper(client.Transport, Policy{
		MaxRetries:    2,
		MinWait:       1 * time.Millisecond,
		MaxWait:       5 * time.Millisecond,
		RetryOnStatus: []int{http.StatusGatewayTimeout},
	})

	reqPayload := "important payload data"
	resp, err := client.Post(srv.URL, "text/plain", bytes.NewBufferString(reqPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(receivedBodies) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(receivedBodies))
	}
	for i, b := range receivedBodies {
		if b != reqPayload {
			t.Errorf("attempt %d: got payload %q, want %q", i+1, b, reqPayload)
		}
	}
}

func TestRoundTrip_ContextCanceled_AbortsEarly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Transport = NewRoundTripper(client.Transport, Policy{
		MaxRetries:    3,
		MinWait:       500 * time.Millisecond,
		MaxWait:       1 * time.Second,
		RetryOnStatus: []int{http.StatusInternalServerError},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected error due to context cancellation, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context error, got %v", err)
	}
}
