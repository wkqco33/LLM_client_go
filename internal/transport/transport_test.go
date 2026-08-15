package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wkqco33/LLM_client_go/retry"
)

type sampleRequest struct {
	Query string `json:"query"`
}

type sampleResponse struct {
	Answer string `json:"answer"`
}

func TestBuildHTTPClient(t *testing.T) {
	t.Run("default with timeout and default policy", func(t *testing.T) {
		hc := BuildHTTPClient(nil, 5*time.Second, nil)
		if hc == nil {
			t.Fatal("expected non-nil http.Client")
		}
		if hc.Timeout != 5*time.Second {
			t.Errorf("got timeout %v, want 5s", hc.Timeout)
		}
		if _, ok := hc.Transport.(*retry.RoundTripper); !ok {
			t.Errorf("expected Transport to be wrapped in *retry.RoundTripper")
		}
	})

	t.Run("custom client with custom policy", func(t *testing.T) {
		custom := &http.Client{Timeout: 10 * time.Second}
		policy := &retry.Policy{MaxRetries: 1}
		hc := BuildHTTPClient(custom, 5*time.Second, policy)
		if hc != custom {
			t.Errorf("expected custom client to be returned")
		}
		if hc.Timeout != 10*time.Second {
			t.Errorf("custom timeout should be preserved, got %v", hc.Timeout)
		}
	})
}

func TestDo_And_DecodeJSON_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"answer":"42"}`))
	}))
	t.Cleanup(srv.Close)

	hc := srv.Client()
	resp, err := Do(context.Background(), hc, "test-provider", http.MethodPost, srv.URL, sampleRequest{Query: "question"}, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer test-token")
	})
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}

	decoded, err := DecodeJSON[sampleResponse]("test-provider", resp, func(r *http.Response) error {
		defer r.Body.Close()
		return errors.New("parse error")
	})
	if err != nil {
		t.Fatalf("DecodeJSON failed: %v", err)
	}
	if decoded.Answer != "42" {
		t.Errorf("got answer %q, want %q", decoded.Answer, "42")
	}
}

func TestDecodeJSON_Non200_DelegatesToParseErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	t.Cleanup(srv.Close)

	hc := srv.Client()
	resp, err := Do(context.Background(), hc, "test-provider", http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}

	expectedErr := errors.New("custom not found error")
	_, err = DecodeJSON[sampleResponse]("test-provider", resp, func(r *http.Response) error {
		defer r.Body.Close()
		_, _ = io.ReadAll(r.Body)
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}
