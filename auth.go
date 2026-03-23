package indigo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type tokenManager struct {
	mu           sync.Mutex
	httpClient   *http.Client
	baseURL      string
	clientID     string
	clientSecret string
	logger       *slog.Logger

	token     string
	expiresAt time.Time
}

// Token returns a valid bearer token, refreshing if needed.
// Safe to call concurrently.
func (m *tokenManager) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.token != "" && time.Now().Before(m.expiresAt.Add(-60*time.Second)) {
		return m.token, nil
	}
	return m.refresh(ctx)
}

// refresh fetches a new token from the API. Caller must hold m.mu.
func (m *tokenManager) refresh(ctx context.Context) (string, error) {
	body, err := json.Marshal(map[string]string{
		"grantType":    "client_credentials",
		"clientId":     m.clientID,
		"clientSecret": m.clientSecret,
		"code":         "",
	})
	if err != nil {
		return "", fmt.Errorf("indigo: auth: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/oauth/v1/accesstokens", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("indigo: auth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("indigo: auth: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("indigo: auth: read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(raw),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	var tokenResp accessTokenResponse
	if err := json.Unmarshal(raw, &tokenResp); err != nil {
		return "", fmt.Errorf("indigo: auth: decode response: %w", err)
	}

	expiresInSec, err := strconv.Atoi(tokenResp.ExpiresIn)
	if err != nil {
		return "", fmt.Errorf("indigo: auth: parse expiresIn %q: %w", tokenResp.ExpiresIn, err)
	}

	issuedAtMs, err := strconv.ParseInt(tokenResp.IssuedAt, 10, 64)
	if err != nil {
		return "", fmt.Errorf("indigo: auth: parse issuedAt %q: %w", tokenResp.IssuedAt, err)
	}

	issuedAt := time.UnixMilli(issuedAtMs)
	m.token = tokenResp.AccessToken
	m.expiresAt = issuedAt.Add(time.Duration(expiresInSec) * time.Second)

	m.logger.DebugContext(ctx, "indigo: token refreshed", slog.Time("expires_at", m.expiresAt))

	return m.token, nil
}
