package indigo

import (
	"net/http"
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("id", "secret")
	if c.baseURL != "https://api.customer.jp" {
		t.Errorf("baseURL = %q, want production URL", c.baseURL)
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", c.httpClient.Timeout)
	}
	if c.retry.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", c.retry.MaxAttempts)
	}
	if c.logger == nil {
		t.Fatal("logger is nil")
	}
	if c.rng == nil {
		t.Fatal("rng is nil")
	}
}

func TestWithBaseURL(t *testing.T) {
	c := NewClient("id", "secret", WithBaseURL("https://example.com"))
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want https://example.com", c.baseURL)
	}
}

func TestWithTimeout(t *testing.T) {
	c := NewClient("id", "secret", WithTimeout(10*time.Second))
	if c.httpClient.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", c.httpClient.Timeout)
	}
}

func TestWithHTTPClient(t *testing.T) {
	hc := &http.Client{Timeout: 99 * time.Second}
	c := NewClient("id", "secret", WithHTTPClient(hc))
	if c.httpClient != hc {
		t.Error("httpClient not replaced by WithHTTPClient")
	}
}

func TestWithLogger(t *testing.T) {
	l := noopLogger()
	c := NewClient("id", "secret", WithLogger(l))
	if c.logger != l {
		t.Error("logger not set by WithLogger")
	}
}

func TestWithRetryConfig(t *testing.T) {
	r := RetryConfig{MaxAttempts: 7, BaseDelay: 1, MaxDelay: 2, Multiplier: 3}
	c := NewClient("id", "secret", WithRetryConfig(r))
	if c.retry.MaxAttempts != 7 {
		t.Errorf("MaxAttempts = %d, want 7", c.retry.MaxAttempts)
	}
}
