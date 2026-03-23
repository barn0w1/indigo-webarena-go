// Package indigo provides a Go client for the WebARENA Indigo VPS API.
package indigo

import (
	"log/slog"
	"math/rand"
	"net/http"
	"time"
)

// Client is the entry point for the WebARENA Indigo API.
// Access services via the SSH and Instance fields.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      *tokenManager
	retry      RetryConfig
	logger     *slog.Logger
	rng        *rand.Rand

	SSH      SSHKeyService
	Instance InstanceService
}

// Option is a functional option for configuring a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

// WithHTTPClient supplies a custom *http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithLogger sets the slog.Logger used by the client.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) { c.logger = l }
}

// WithRetryConfig overrides the default retry configuration.
func WithRetryConfig(r RetryConfig) Option {
	return func(c *Client) { c.retry = r }
}

// WithTimeout sets the timeout on the default HTTP client.
// It has no effect if WithHTTPClient was also used.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// NewClient constructs a Client authenticated with the given credentials.
// No I/O is performed; the first API call triggers the initial token fetch.
func NewClient(clientID, clientSecret string, opts ...Option) *Client {
	c := &Client{
		baseURL:    "https://api.customer.jp",
		httpClient: &http.Client{Timeout: 30 * time.Second},
		retry:      DefaultRetryConfig(),
		logger:     slog.Default(),
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
	}
	for _, opt := range opts {
		opt(c)
	}
	c.token = &tokenManager{
		httpClient:   c.httpClient,
		baseURL:      c.baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		logger:       c.logger,
	}
	c.SSH = SSHKeyService{client: c}
	c.Instance = InstanceService{client: c}
	return c
}
