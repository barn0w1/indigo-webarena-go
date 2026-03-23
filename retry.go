package indigo

import (
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryConfig controls retry and exponential backoff behaviour.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
	}
}

// delay returns the wait duration before the given attempt (0-indexed).
// Adds ±25% jitter to prevent thundering herd.
func (c RetryConfig) delay(attempt int, rng *rand.Rand) time.Duration {
	d := float64(c.BaseDelay) * math.Pow(c.Multiplier, float64(attempt))
	if d > float64(c.MaxDelay) {
		d = float64(c.MaxDelay)
	}
	jitter := d * 0.25 * (2*rng.Float64() - 1)
	return time.Duration(d + jitter)
}

// isRetryable reports whether the response warrants a retry.
func isRetryable(statusCode int, err error) bool {
	if err != nil {
		return true
	}
	return statusCode == http.StatusTooManyRequests ||
		(statusCode >= 500 && statusCode <= 599)
}
