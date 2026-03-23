package indigo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// addQueryInt appends key=val to q only when val is non-zero.
func addQueryInt(q url.Values, key string, val int) {
	if val != 0 {
		q.Set(key, strconv.Itoa(val))
	}
}

// doRaw executes a single HTTP request. The caller is responsible for closing
// the response body. If authenticated is true, a bearer token is injected.
func (c *Client) doRaw(ctx context.Context, req *http.Request, authenticated bool) (*http.Response, error) {
	if authenticated {
		token, err := c.token.Token(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	start := time.Now()
	c.logger.DebugContext(ctx, "indigo: request",
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	c.logger.DebugContext(ctx, "indigo: response",
		slog.Int("status", resp.StatusCode),
		slog.Duration("elapsed", time.Since(start)),
	)
	return resp, nil
}

// do executes a request with retry logic. body is serialised once and replayed
// on each attempt. The decoded response is written into dst when dst is non-nil.
func (c *Client) do(ctx context.Context, method, path string, body interface{}, dst interface{}, authenticated bool) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("indigo: marshal request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt < c.retry.MaxAttempts; attempt++ {
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
		if err != nil {
			return fmt.Errorf("indigo: build request: %w", err)
		}

		resp, err := c.doRaw(ctx, req, authenticated)
		if err != nil {
			lastErr = err
			if !isRetryable(0, err) || attempt == c.retry.MaxAttempts-1 {
				return fmt.Errorf("indigo: %w", err)
			}
			if werr := c.waitBackoff(ctx, attempt); werr != nil {
				return werr
			}
			continue
		}

		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("indigo: read response: %w", readErr)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := &APIError{
				StatusCode: resp.StatusCode,
				Body:       string(raw),
				RequestID:  resp.Header.Get("X-Request-ID"),
			}
			lastErr = apiErr
			if !isRetryable(resp.StatusCode, nil) || attempt == c.retry.MaxAttempts-1 {
				return apiErr
			}
			c.logger.InfoContext(ctx, "indigo: retrying request",
				slog.Int("attempt", attempt+1),
				slog.Int("status", resp.StatusCode),
			)
			if werr := c.waitBackoff(ctx, attempt); werr != nil {
				return werr
			}
			continue
		}

		if dst != nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, dst); err != nil {
				return fmt.Errorf("indigo: decode response: %w", err)
			}
		}
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return nil
}

// waitBackoff sleeps for the retry backoff duration, respecting ctx cancellation.
func (c *Client) waitBackoff(ctx context.Context, attempt int) error {
	wait := c.retry.delay(attempt, c.rng)
	c.logger.InfoContext(ctx, "indigo: backoff",
		slog.Int("attempt", attempt+1),
		slog.Duration("wait", wait),
	)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}
