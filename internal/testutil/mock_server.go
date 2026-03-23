package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
)

// MockServer is a test HTTP server that records received requests and dispatches
// them to registered handlers. The token endpoint is pre-registered and always
// returns a dummy token with a long TTL so tests do not need to stub auth
// unless they are explicitly testing authentication behaviour.
type MockServer struct {
	Server   *httptest.Server
	mu       sync.Mutex
	requests []*http.Request
}

// NewMockServer creates a MockServer whose handlers are keyed by "METHOD /path".
// Unmatched requests receive a 404 response. The POST /oauth/v1/accesstokens
// endpoint is always registered automatically.
func NewMockServer(handlers map[string]http.HandlerFunc) *MockServer {
	ms := &MockServer{}

	mux := http.NewServeMux()

	// Always provide a working token endpoint.
	mux.HandleFunc("POST /oauth/v1/accesstokens", func(w http.ResponseWriter, r *http.Request) {
		ms.record(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"accessToken": "test-token",
			"tokenType":   "BearerToken",
			"expiresIn":   "3599",
			"issuedAt":    "9999999999000", // far future: year ~2286
		})
	})

	for pattern, h := range handlers {
		h := h // capture
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			ms.record(r)
			h(w, r)
		})
	}

	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
	}))
	return ms
}

func (m *MockServer) record(r *http.Request) {
	m.mu.Lock()
	m.requests = append(m.requests, r)
	m.mu.Unlock()
}

// Close shuts down the test server.
func (m *MockServer) Close() {
	m.Server.Close()
}

// RequestCount returns the number of requests received for the given method and path.
func (m *MockServer) RequestCount(method, path string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, r := range m.requests {
		if r.Method == method && r.URL.Path == path {
			count++
		}
	}
	return count
}

// LastRequest returns the most recent request for the given method and path, or nil.
func (m *MockServer) LastRequest(method, path string) *http.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.requests) - 1; i >= 0; i-- {
		r := m.requests[i]
		if r.Method == method && r.URL.Path == path {
			return r
		}
	}
	return nil
}
