package indigo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func newTestTokenManager(serverURL string, hc *http.Client) *tokenManager {
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	return &tokenManager{
		httpClient:   hc,
		baseURL:      serverURL,
		clientID:     "test-id",
		clientSecret: "test-secret",
		logger:       noopLogger(),
	}
}

// futureIssuedAt is a Unix-millisecond timestamp in the far future (year ~2286),
// ensuring tokens returned by test servers appear unexpired.
const futureIssuedAt = "9999999999000"

func tokenHandler(expiresIn, issuedAt string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"accessToken": "tok-abc",
			"tokenType":   "BearerToken",
			"expiresIn":   expiresIn,
			"issuedAt":    issuedAt,
		})
	}
}

func TestTokenManagerFetch(t *testing.T) {
	srv := httptest.NewServer(tokenHandler("3599", futureIssuedAt))
	defer srv.Close()

	tm := newTestTokenManager(srv.URL, nil)
	tok, err := tm.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok != "tok-abc" {
		t.Errorf("got token %q, want %q", tok, "tok-abc")
	}
}

func TestTokenManagerCacheHit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		tokenHandler("3599", futureIssuedAt)(w, r)
	}))
	defer srv.Close()

	tm := newTestTokenManager(srv.URL, nil)
	for range 3 {
		if _, err := tm.Token(context.Background()); err != nil {
			t.Fatalf("Token() error: %v", err)
		}
	}
	if calls != 1 {
		t.Errorf("server called %d times, want 1", calls)
	}
}

func TestTokenManagerRefreshOnExpiry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// issuedAt in the past, expiresIn=0 → already expired
		tokenHandler("0", futureIssuedAt)(w, r)
	}))
	defer srv.Close()

	tm := newTestTokenManager(srv.URL, nil)
	// First call fetches.
	if _, err := tm.Token(context.Background()); err != nil {
		t.Fatalf("first Token() error: %v", err)
	}
	// Force expiry by clearing the cached expiry time.
	tm.mu.Lock()
	tm.expiresAt = time.Now().Add(-1 * time.Second)
	tm.mu.Unlock()

	// Second call should refresh.
	if _, err := tm.Token(context.Background()); err != nil {
		t.Fatalf("second Token() error: %v", err)
	}
	if calls != 2 {
		t.Errorf("server called %d times, want 2", calls)
	}
}

func TestTokenManagerNon201Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	tm := newTestTokenManager(srv.URL, nil)
	_, err := tm.Token(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
}

func TestTokenManagerBadExpiresIn(t *testing.T) {
	srv := httptest.NewServer(tokenHandler("notanumber", futureIssuedAt))
	defer srv.Close()

	tm := newTestTokenManager(srv.URL, nil)
	_, err := tm.Token(context.Background())
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestTokenManagerBadIssuedAt(t *testing.T) {
	srv := httptest.NewServer(tokenHandler("3599", "notanumber"))
	defer srv.Close()

	tm := newTestTokenManager(srv.URL, nil)
	_, err := tm.Token(context.Background())
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestTokenManagerConcurrent(t *testing.T) {
	calls := 0
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		tokenHandler("3599", futureIssuedAt)(w, r)
	}))
	defer srv.Close()

	tm := newTestTokenManager(srv.URL, nil)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tm.Token(context.Background()); err != nil {
				t.Errorf("Token() error: %v", err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	c := calls
	mu.Unlock()
	if c != 1 {
		t.Errorf("server called %d times under concurrency, want 1", c)
	}
}
