package indigo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(serverURL string) *Client {
	return NewClient("id", "secret",
		WithBaseURL(serverURL),
		WithLogger(noopLogger()),
		WithRetryConfig(RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   1 * time.Millisecond,
			MaxDelay:    5 * time.Millisecond,
			Multiplier:  2.0,
		}),
	)
}

func tokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/v1/accesstokens" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
				"accessToken": "test-token",
				"tokenType":   "BearerToken",
				"expiresIn":   "3599",
				"issuedAt":    "1000000000000",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TestDo2xxDecodesJSON(t *testing.T) {
	type payload struct {
		Value string `json:"value"`
	}
	srv := httptest.NewServer(tokenMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload{Value: "hello"}) //nolint:errcheck
	})))
	defer srv.Close()

	c := newTestClient(srv.URL)
	var dst payload
	if err := c.do(context.Background(), http.MethodGet, "/test", nil, &dst, true); err != nil {
		t.Fatalf("do() error: %v", err)
	}
	if dst.Value != "hello" {
		t.Errorf("got %q, want %q", dst.Value, "hello")
	}
}

func TestDo500IsRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(tokenMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" {
			calls++
			http.Error(w, "oops", http.StatusInternalServerError)
		}
	})))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.do(context.Background(), http.MethodGet, "/test", nil, nil, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 3 {
		t.Errorf("server called %d times, want 3", calls)
	}
}

func TestDo429IsRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(tokenMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" {
			calls++
			w.WriteHeader(http.StatusTooManyRequests)
		}
	})))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.do(context.Background(), http.MethodGet, "/test", nil, nil, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 3 {
		t.Errorf("server called %d times, want 3", calls)
	}
}

func TestDo400NotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(tokenMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" {
			calls++
			http.Error(w, "bad request", http.StatusBadRequest)
		}
	})))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.do(context.Background(), http.MethodGet, "/test", nil, nil, true)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if calls != 1 {
		t.Errorf("server called %d times, want 1", calls)
	}
}

func TestDoContextCancellation(t *testing.T) {
	srv := httptest.NewServer(tokenMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer srv.Close()

	c := NewClient("id", "secret",
		WithBaseURL(srv.URL),
		WithLogger(noopLogger()),
		WithRetryConfig(RetryConfig{
			MaxAttempts: 5,
			BaseDelay:   500 * time.Millisecond,
			MaxDelay:    2 * time.Second,
			Multiplier:  2.0,
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := c.do(ctx, http.MethodGet, "/test", nil, nil, true)
	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestDoAuthHeaderPresent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(tokenMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}
	})))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_ = c.do(context.Background(), http.MethodGet, "/test", nil, nil, true)
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-token")
	}
}

func TestDoAuthHeaderAbsentWhenUnauthenticated(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_ = c.do(context.Background(), http.MethodGet, "/test", nil, nil, false)
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty", gotAuth)
	}
}
