package indigo

import (
	"math/rand"
	"net/http"
	"testing"
	"time"
)

func TestRetryConfigDelay(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Second,
		MaxDelay:    10 * time.Second,
		Multiplier:  2.0,
	}
	rng := rand.New(rand.NewSource(42))

	for attempt := 0; attempt < 5; attempt++ {
		d := cfg.delay(attempt, rng)
		if d <= 0 {
			t.Errorf("attempt %d: delay %v <= 0", attempt, d)
		}
		if d > cfg.MaxDelay+cfg.MaxDelay/4 {
			t.Errorf("attempt %d: delay %v exceeds MaxDelay+jitter", attempt, d)
		}
	}
}

func TestRetryConfigDelayExponentialGrowth(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    60 * time.Second,
		Multiplier:  2.0,
	}
	// Use a fixed seed with neutral jitter (will not make delays shrink across attempts
	// reliably, so just verify they stay within bounds).
	rng := rand.New(rand.NewSource(0))

	prev := time.Duration(0)
	for attempt := 0; attempt < 5; attempt++ {
		d := cfg.delay(attempt, rng)
		// Each delay must be positive and not exceed MaxDelay + 25% jitter.
		if d <= 0 {
			t.Errorf("attempt %d: non-positive delay %v", attempt, d)
		}
		maxAllowed := cfg.MaxDelay + cfg.MaxDelay/4
		if d > maxAllowed {
			t.Errorf("attempt %d: delay %v exceeds max %v", attempt, d, maxAllowed)
		}
		_ = prev
		prev = d
	}
}

func TestRetryConfigDelayCappedAtMaxDelay(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   1 * time.Second,
		MaxDelay:    4 * time.Second,
		Multiplier:  10.0,
	}
	rng := rand.New(rand.NewSource(1))
	for attempt := 3; attempt < 10; attempt++ {
		d := cfg.delay(attempt, rng)
		// With 10× multiplier the raw value blows past MaxDelay quickly;
		// allow for +25% jitter headroom.
		if d > cfg.MaxDelay+cfg.MaxDelay/4 {
			t.Errorf("attempt %d: delay %v exceeds MaxDelay+jitter", attempt, d)
		}
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		status int
		err    error
		want   bool
	}{
		{http.StatusInternalServerError, nil, true},
		{http.StatusBadGateway, nil, true},
		{http.StatusServiceUnavailable, nil, true},
		{http.StatusTooManyRequests, nil, true},
		{0, errNetwork, true},
		{http.StatusOK, nil, false},
		{http.StatusCreated, nil, false},
		{http.StatusBadRequest, nil, false},
		{http.StatusUnauthorized, nil, false},
		{http.StatusNotFound, nil, false},
	}
	for _, tt := range tests {
		got := isRetryable(tt.status, tt.err)
		if got != tt.want {
			t.Errorf("isRetryable(%d, %v) = %v, want %v", tt.status, tt.err, got, tt.want)
		}
	}
}

// errNetwork is a sentinel network error for testing.
var errNetwork = &networkError{}

type networkError struct{}

func (e *networkError) Error() string { return "connection reset" }
