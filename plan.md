# Implementation Plan: WebARENA Indigo Go SDK

## Overview

A production-quality, library-grade Go client SDK for the WebARENA Indigo VPS API.
Zero third-party runtime dependencies. Fully goroutine-safe. Context-aware throughout.

---

## File-by-File Breakdown

### `go.mod`

Already present. No changes needed at the library level. Test dependencies (`net/http/httptest`
is stdlib) mean `go.sum` stays empty unless we add a test framework. We will not add one —
standard `testing` package is sufficient.

---

### `models.go` — All public types and enum constants

This is the foundation everything else builds on. Define it first.

#### Enum types

Raw strings must not leak into the public API. All enumerations are typed string constants:

```go
// InstanceAction is the operation to apply to an instance.
type InstanceAction string

const (
    InstanceActionStart     InstanceAction = "start"
    InstanceActionStop      InstanceAction = "stop"
    InstanceActionForceStop InstanceAction = "forcestop"
    InstanceActionReset     InstanceAction = "reset"
    InstanceActionDestroy   InstanceAction = "destroy"
)

// SSHKeyStatus represents the activation state of an SSH key.
type SSHKeyStatus string

const (
    SSHKeyStatusActive   SSHKeyStatus = "ACTIVE"
    SSHKeyStatusDeactive SSHKeyStatus = "DEACTIVE"
)
```

#### Request structs

```go
type CreateSSHKeyRequest struct {
    Name      string // sshName
    PublicKey string // sshKey
}

type UpdateSSHKeyRequest struct {
    Name      string       // sshName (omitempty)
    PublicKey string       // sshKey  (omitempty)
    Status    SSHKeyStatus // sshKeyStatus (omitempty)
}

type CreateInstanceRequest struct {
    Name       string         // instanceName (required)
    Plan       int            // instancePlan (required)
    RegionID   int            // regionId (omitempty)
    OSID       int            // osId (omitempty)
    SSHKeyID   int            // sshKeyId (omitempty)
    WinPassword string        // winPassword (omitempty)
    ImportURL  string         // importUrl (omitempty)
    SnapshotID int            // snapshotId (omitempty)
}
```

Note: JSON tags use `omitempty` on optional fields. Zero-value integers (0) must be omitted
so the API doesn't receive spurious 0 values for optional resource IDs. We'll use `*int`
for optional integer fields (RegionID, OSID, SSHKeyID, SnapshotID) so the zero value is
distinguishable from "not provided."

Revised:
```go
type CreateInstanceRequest struct {
    Name        string  `json:"instanceName"`
    Plan        int     `json:"instancePlan"`
    RegionID    *int    `json:"regionId,omitempty"`
    OSID        *int    `json:"osId,omitempty"`
    SSHKeyID    *int    `json:"sshKeyId,omitempty"`
    WinPassword string  `json:"winPassword,omitempty"`
    ImportURL   string  `json:"importUrl,omitempty"`
    SnapshotID  *int    `json:"snapshotId,omitempty"`
}
```

#### Response structs

The token response has two spec quirks to address:

```go
// accessTokenResponse is the raw JSON response — unexported because callers never see it.
type accessTokenResponse struct {
    AccessToken string `json:"accessToken"`
    TokenType   string `json:"tokenType"`
    ExpiresIn   string `json:"expiresIn"` // spec quirk: string, not int ("3599")
    Scope       string `json:"scope"`
    IssuedAt    string `json:"issuedAt"`  // spec quirk: Unix ms as string ("1550570350202")
}
```

Parsing logic (in `auth.go`):
```go
expiresInSec, err := strconv.Atoi(resp.ExpiresIn)   // "3599" → 3599
issuedAtMs, err   := strconv.ParseInt(resp.IssuedAt, 10, 64) // "1550570350202"
issuedAt          := time.UnixMilli(issuedAtMs)
expiresAt         := issuedAt.Add(time.Duration(expiresInSec) * time.Second)
```

Public SSH key and instance models mirror the spec schemas exactly:

```go
type SSHKey struct {
    ID        int          `json:"id"`
    ServiceID string       `json:"service_id"`
    UserID    int          `json:"user_id"`
    Name      string       `json:"name"`
    PublicKey string       `json:"sshkey"`
    Status    SSHKeyStatus `json:"status"`
    CreatedAt time.Time    `json:"created_at"`
    UpdatedAt time.Time    `json:"updated_at"`
}

type InstanceType struct {
    ID          int    `json:"id"`
    Name        string `json:"name"`
    DisplayName string `json:"display_name"`
}

type Region struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type InstanceSpec struct {
    ID          int    `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

type Instance struct {
    ID           int    `json:"id"`
    Name         string `json:"instance_name"`
    Status       string `json:"status"`
    SSHKeyID     int    `json:"sshkey_id"`
    HostID       int    `json:"host_id"`
    Plan         string `json:"plan"`
    MemSize      int    `json:"memsize"`
    CPUs         int    `json:"cpus"`
    OSID         int    `json:"os_id"`
    UUID         string `json:"uuid"`
    IP           string `json:"ip"`
    ArpaName     string `json:"arpaname"`
}

type UpdateInstanceStatusResult struct {
    Success        bool   `json:"success"`
    Message        string `json:"message"`
    SuccessCode    string `json:"successCode"`
    InstanceStatus string `json:"instanceStatus"`
}
```

---

### `errors.go` — Structured error types

Callers must be able to distinguish API errors from network errors and inspect HTTP status codes.

```go
// APIError represents a non-2xx response from the WebARENA Indigo API.
type APIError struct {
    StatusCode int
    Body       string  // raw response body for debugging
    RequestID  string  // if the API ever returns X-Request-ID
}

func (e *APIError) Error() string {
    return fmt.Sprintf("indigo: API error %d: %s", e.StatusCode, e.Body)
}

// IsNotFound returns true if the error is a 404 API error.
func IsNotFound(err error) bool {
    var apiErr *APIError
    return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// IsUnauthorized returns true if the error is a 401 API error.
func IsUnauthorized(err error) bool {
    var apiErr *APIError
    return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized
}
```

Callers use `errors.As(err, &apiErr)` to inspect the status code and body. Helper predicates
(`IsNotFound`, `IsUnauthorized`) are pure convenience — they do not hide the underlying type.

---

### `retry.go` — RetryConfig and backoff algorithm

Fully self-contained. No dependency on the rest of the package.

```go
// RetryConfig controls the retry and backoff behaviour.
type RetryConfig struct {
    MaxAttempts int           // default 3
    BaseDelay   time.Duration // default 500ms
    MaxDelay    time.Duration // default 30s
    Multiplier  float64       // default 2.0
}

func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   500 * time.Millisecond,
        MaxDelay:    30 * time.Second,
        Multiplier:  2.0,
    }
}

// delay returns the wait time before attempt n (0-indexed).
// Adds ±25% jitter to prevent thundering herd.
func (c RetryConfig) delay(attempt int) time.Duration {
    d := float64(c.BaseDelay) * math.Pow(c.Multiplier, float64(attempt))
    if d > float64(c.MaxDelay) {
        d = float64(c.MaxDelay)
    }
    // jitter: ±25%
    jitter := d * 0.25 * (2*rand.Float64() - 1)
    return time.Duration(d + jitter)
}

// isRetryable returns true for conditions that warrant a retry.
func isRetryable(statusCode int, err error) bool {
    if err != nil {
        // Retry on network-level errors (timeouts, connection resets)
        return true
    }
    return statusCode == http.StatusTooManyRequests ||
        (statusCode >= 500 && statusCode <= 599)
}
```

The delay function uses `math/rand` (seeded via `rand.New(rand.NewSource(time.Now().UnixNano()))`
at client construction) — no `crypto/rand` needed here.

---

### `auth.go` — Token cache and auto-refresh

The token manager is the most complex stateful piece. It must be goroutine-safe.

```go
// tokenManager fetches, caches, and auto-refreshes the bearer token.
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
// It is safe to call concurrently.
func (m *tokenManager) Token(ctx context.Context) (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Refresh 60 seconds before actual expiry to avoid races with in-flight requests.
    if time.Now().Before(m.expiresAt.Add(-60 * time.Second)) {
        return m.token, nil
    }

    return m.refresh(ctx)
}

// refresh performs the actual POST /oauth/v1/accesstokens call.
// Caller must hold m.mu.
func (m *tokenManager) refresh(ctx context.Context) (string, error) { ... }
```

Key design decisions:
- A single `sync.Mutex` guards both reads and writes. Token fetches are infrequent (once per
  hour), so the contention cost of a full lock on every `Token()` call is negligible.
- The 60-second pre-expiry buffer ensures tokens do not expire mid-flight on slow networks.
- `refresh` is called only while holding the lock, preventing duplicate concurrent fetches
  (double-checked locking is not needed because we hold the lock through the check).

---

### `http.go` — Core HTTP transport, do(), and logging middleware

All HTTP dispatch goes through a single unexported `do()` method. This is where retry,
logging, auth injection, and error unpacking all happen.

```go
// doRaw executes a single HTTP request without retry. Returns (*http.Response, error).
// The caller is responsible for closing the response body.
func (c *Client) doRaw(ctx context.Context, req *http.Request, authenticated bool) (*http.Response, error)

// do executes a request with retry logic. It decodes a successful response into dst
// (if dst != nil), and returns an *APIError for non-2xx status codes.
func (c *Client) do(ctx context.Context, method, path string, body any, dst any, authenticated bool) error
```

The retry loop structure:

```go
for attempt := 0; attempt < c.retry.MaxAttempts; attempt++ {
    // Re-serialize body for each attempt (body reader is consumed after first use).
    // Serialize once, store as []byte, wrap in bytes.NewReader per attempt.

    resp, err := c.doRaw(ctx, req, authenticated)

    if !isRetryable(statusCode, err) || attempt == c.retry.MaxAttempts-1 {
        break
    }

    wait := c.retry.delay(attempt)
    c.logger.InfoContext(ctx, "retrying request",
        slog.Int("attempt", attempt+1),
        slog.Int("status", statusCode),
        slog.Duration("backoff", wait),
    )

    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(wait):
    }
}
```

Logging via `log/slog`:
- Each outgoing request logs at `DEBUG`: method, path, attempt number.
- Each response logs at `DEBUG`: status, duration.
- Retries log at `INFO`.
- Non-retryable errors log at `WARN`.
- The client accepts a `*slog.Logger` and defaults to `slog.Default()` if nil.

Query parameter helpers in `http.go`:

```go
// addQueryInt appends a query parameter only if val > 0, avoiding zero-value noise.
func addQueryInt(q url.Values, key string, val int) {
    if val > 0 {
        q.Set(key, strconv.Itoa(val))
    }
}
```

---

### `indigo.go` — Client, NewClient, and functional options

The `Client` is the user-facing entry point. It embeds service types as value fields so
callers access them as `client.SSH` and `client.Instance` rather than needing separate
constructor calls.

```go
type Client struct {
    baseURL    string
    httpClient *http.Client
    token      *tokenManager
    retry      RetryConfig
    logger     *slog.Logger

    // Public service handles
    SSH      SSHKeyService
    Instance InstanceService
}

func NewClient(clientID, clientSecret string, opts ...Option) *Client {
    c := &Client{
        baseURL:    "https://api.customer.jp",
        httpClient: &http.Client{Timeout: 30 * time.Second},
        retry:      DefaultRetryConfig(),
        logger:     slog.Default(),
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
```

Functional options:

```go
type Option func(*Client)

// WithBaseURL overrides the API base URL (useful for testing).
func WithBaseURL(u string) Option

// WithHTTPClient supplies a custom *http.Client (e.g., with a custom transport).
func WithHTTPClient(hc *http.Client) Option

// WithLogger sets the slog.Logger for the client.
func WithLogger(l *slog.Logger) Option

// WithRetryConfig overrides the default retry configuration.
func WithRetryConfig(r RetryConfig) Option

// WithTimeout sets a timeout on the default HTTP client.
// Has no effect if WithHTTPClient has been used.
func WithTimeout(d time.Duration) Option
```

`NewClient` does NOT eagerly fetch a token. The first API call triggers the initial token
fetch. This keeps `NewClient` free of I/O and makes it safe to call at program startup.

---

### `sshkey.go` — SSHKeyService

```go
// SSHKeyService wraps all SSH key endpoints.
type SSHKeyService struct {
    client *Client
}

func (s SSHKeyService) List(ctx context.Context) ([]SSHKey, error)
func (s SSHKeyService) ListActive(ctx context.Context) ([]SSHKey, error)
func (s SSHKeyService) Get(ctx context.Context, id int) (*SSHKey, error)
func (s SSHKeyService) Create(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error)
func (s SSHKeyService) Update(ctx context.Context, id int, req UpdateSSHKeyRequest) error
func (s SSHKeyService) Delete(ctx context.Context, id int) error
```

Each method calls `c.do()`, passing a typed struct for decoding. The internal envelope types
(e.g., `{"success": true, "sshkeys": [...]}`) are unexported local structs defined inline:

```go
func (s SSHKeyService) List(ctx context.Context) ([]SSHKey, error) {
    var envelope struct {
        Success bool     `json:"success"`
        Total   int      `json:"total"`
        SSHKeys []SSHKey `json:"sshkeys"`
    }
    if err := s.client.do(ctx, http.MethodGet, "/webarenaIndigo/v1/vm/sshkey", nil, &envelope, true); err != nil {
        return nil, err
    }
    return envelope.SSHKeys, nil
}
```

---

### `instance.go` — InstanceService

```go
// InstanceService wraps all instance endpoints.
type InstanceService struct {
    client *Client
}

func (s InstanceService) ListTypes(ctx context.Context) ([]InstanceType, error)
func (s InstanceService) ListRegions(ctx context.Context, instanceTypeID int) ([]Region, error)
func (s InstanceService) ListOS(ctx context.Context, instanceTypeID int) ([]OSCategory, error)
func (s InstanceService) ListSpecs(ctx context.Context, instanceTypeID, osID int) ([]InstanceSpec, error)
func (s InstanceService) Create(ctx context.Context, req CreateInstanceRequest) (*Instance, error)
func (s InstanceService) List(ctx context.Context) ([]Instance, error)
func (s InstanceService) UpdateStatus(ctx context.Context, instanceID string, action InstanceAction) (*UpdateInstanceStatusResult, error)

// Convenience wrappers over UpdateStatus:
func (s InstanceService) Start(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error)
func (s InstanceService) Stop(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error)
func (s InstanceService) ForceStop(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error)
func (s InstanceService) Reset(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error)
func (s InstanceService) Destroy(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error)
```

Optional query parameters are handled by building `url.Values` and calling `addQueryInt`:

```go
func (s InstanceService) ListRegions(ctx context.Context, instanceTypeID int) ([]Region, error) {
    q := url.Values{}
    addQueryInt(q, "instanceTypeId", instanceTypeID)
    path := "/webarenaIndigo/v1/vm/getregion"
    if len(q) > 0 {
        path += "?" + q.Encode()
    }
    // ... do() call
}
```

Passing `0` for optional integer parameters means "no filter" — the `addQueryInt` helper
silently drops the parameter, matching the spec's `required: false` semantics.

---

### `internal/testutil/mock_server.go` — Test infrastructure

An `httptest.Server` wrapper that records requests and lets tests register handler stubs.

```go
// MockServer is a test HTTP server with helpers for asserting API calls.
type MockServer struct {
    Server   *httptest.Server
    Requests []*http.Request // all received requests, in order
    mu       sync.Mutex
}

// NewMockServer creates a MockServer with the given handler map.
// handlers maps "<METHOD> <path>" (e.g., "GET /webarenaIndigo/v1/vm/sshkey") to
// a handler function that writes the response.
func NewMockServer(handlers map[string]http.HandlerFunc) *MockServer

// NewClient returns a Client pre-configured to talk to the mock server.
// It pre-seeds a dummy token so tests don't need to stub the auth endpoint
// unless they're testing auth behaviour specifically.
func (m *MockServer) NewClient() *Client

// Close shuts down the test server.
func (m *MockServer) Close()
```

Usage in tests:

```go
func TestSSHKeyList(t *testing.T) {
    ms := testutil.NewMockServer(map[string]http.HandlerFunc{
        "GET /webarenaIndigo/v1/vm/sshkey": func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]any{
                "success": true,
                "total":   1,
                "sshkeys": []map[string]any{{"id": 1, "name": "my-key", "status": "ACTIVE"}},
            })
        },
    })
    defer ms.Close()

    client := ms.NewClient()
    keys, err := client.SSH.List(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if len(keys) != 1 || keys[0].Name != "my-key" {
        t.Fatalf("unexpected keys: %+v", keys)
    }
}
```

---

## Key Design Decisions and Rationale

### Why value receivers on service types?
`SSHKeyService` and `InstanceService` hold only a single `*Client` pointer. They are safe
to copy. Value receivers avoid an extra indirection and make the zero-value safe (though
callers should use `NewClient`).

### Why not embed services directly on Client?
If `Client` embedded `SSHKeyService`, method names would collide in a flat namespace. The
namespaced `client.SSH.List(...)` pattern is idiomatic in Go SDKs (see `go-github`, the
AWS SDK v2) and avoids future naming conflicts as the API grows.

### Why `*int` for optional integer fields in CreateInstanceRequest?
`0` is a valid JSON value and would be sent to the API. Using pointer fields means the
zero value is `nil`, which marshals to `omitempty`-excluded. This prevents accidents where
a caller forgets to set `RegionID` and sends `regionId: 0`.

### Why inline envelope structs instead of shared envelope types?
Each endpoint returns a slightly different envelope shape (`sshkeys`, `sshKey`,
`instanceTypes`, `regionlist`, etc.). Shared generic envelopes would require generics or
`interface{}` — both reduce readability. Inline structs are slightly repetitive but crystal
clear and cause zero allocations compared to a generic wrapper.

### Why not use `sync.RWMutex` for the token manager?
Token refreshes happen at most once per hour. The vast majority of `Token()` calls hit the
fast path (token is valid) and return immediately. The lock is held for nanoseconds on the
fast path. An `RWMutex` would add complexity with negligible throughput benefit.

### Retry scope
Only the `do()` layer retries — never inside `tokenManager.refresh`. If the token endpoint
itself fails transiently, the upstream `do()` call will handle it because `do()` calls
`token.Token()` at the top of each attempt (re-fetching a fresh token after a 401 could be
added as a future enhancement).

---

## Error Handling Conventions

- Network-level errors (DNS, TCP reset, timeout) are returned unwrapped as `error` — callers
  use `errors.Is(err, context.DeadlineExceeded)` etc. as normal.
- HTTP 4xx/5xx responses are returned as `*APIError`, which callers can inspect via
  `errors.As`.
- Internal parsing errors (malformed JSON, bad token format) are wrapped with `fmt.Errorf("indigo: %w", err)`.
- All error strings are prefixed with `"indigo: "` to make stack traces readable.

---

## Build & Test Plan

1. `go build ./...` — must succeed with zero warnings.
2. `go vet ./...` — must pass.
3. `go test ./...` — cover at minimum:
   - Token fetch, cache hit, cache miss/refresh
   - Retry logic on 500 and 429 responses
   - All SSH key operations (list, get, create, update, delete)
   - All instance operations (list types, regions, OS, specs, create, list, all 5 status actions)
   - `APIError` construction and `IsNotFound` / `IsUnauthorized` helpers
   - Context cancellation propagation
4. `go test -race ./...` — must pass with zero data races.

---

## Implementation Order

1. `models.go` — all types, no logic; easy to validate by inspection
2. `errors.go` — small, foundational
3. `retry.go` — self-contained, easy to unit-test in isolation
4. `auth.go` + `http.go` — core transport; these are co-dependent (auth needs http, http needs auth)
5. `indigo.go` — wires everything together
6. `sshkey.go` — simpler service, good first integration test
7. `instance.go` — more complex service (optional params, action enum)
8. `internal/testutil/mock_server.go` — write alongside or just before tests
9. Tests for all of the above
10. `README.md` — usage examples, configuration reference

---

## Todo List

### Phase 1 — Models (`models.go`)
- [ ] Define `SSHKeyStatus` typed string constant and `ACTIVE` / `DEACTIVE` values
- [ ] Define `InstanceAction` typed string constant and all five values (`start`, `stop`, `forcestop`, `reset`, `destroy`)
- [ ] Define unexported `accessTokenResponse` struct with `ExpiresIn` and `IssuedAt` as `string` fields
- [ ] Define `SSHKey` response struct with all spec fields; use `time.Time` for `created_at` / `updated_at`
- [ ] Define `InstanceType` struct (`id`, `name`, `display_name`)
- [ ] Define `Region` struct (`id`, `name`)
- [ ] Define `InstanceSpec` struct (`id`, `name`, `description`)
- [ ] Define `Instance` struct with all spec fields from `InstanceData` schema
- [ ] Define `UpdateInstanceStatusResult` struct exposing `success`, `message`, `successCode`, `instanceStatus`
- [ ] Define `CreateSSHKeyRequest` struct with `Name` and `PublicKey` string fields and correct JSON tags
- [ ] Define `UpdateSSHKeyRequest` struct with optional `Name`, `PublicKey`, `Status` fields and `omitempty` tags
- [ ] Define `CreateInstanceRequest` struct; use `*int` for optional resource-ID fields (`RegionID`, `OSID`, `SSHKeyID`, `SnapshotID`) with `omitempty`; keep `WinPassword`, `ImportURL` as `string` with `omitempty`
- [ ] Define `ListRegionsParams` and `ListSpecsParams` helper structs (or confirm plain `int` args are sufficient)
- [ ] Add package-level doc comment to `models.go`

### Phase 2 — Errors (`errors.go`)
- [ ] Define `APIError` struct with `StatusCode int`, `Body string`, `RequestID string` fields
- [ ] Implement `(*APIError).Error() string` method; prefix with `"indigo: "`
- [ ] Implement `IsNotFound(err error) bool` helper using `errors.As`
- [ ] Implement `IsUnauthorized(err error) bool` helper using `errors.As`
- [ ] Add package-level doc comment to `errors.go`

### Phase 3 — Retry (`retry.go`)
- [ ] Define `RetryConfig` struct with `MaxAttempts`, `BaseDelay`, `MaxDelay`, `Multiplier` fields
- [ ] Implement `DefaultRetryConfig()` returning sensible defaults (3 attempts, 500 ms base, 30 s max, 2× multiplier)
- [ ] Implement `(RetryConfig).delay(attempt int) time.Duration` with exponential growth capped at `MaxDelay`
- [ ] Add ±25% jitter to `delay` using a local `rand.Rand` seeded at client construction (pass seed or `*rand.Rand` into config, or keep it package-level)
- [ ] Implement `isRetryable(statusCode int, err error) bool`; retry on network errors, 429, and 5xx
- [ ] Write unit tests in `retry_test.go`:
  - [ ] `delay` grows exponentially and never exceeds `MaxDelay`
  - [ ] `delay` output stays within the expected jitter bounds
  - [ ] `isRetryable` returns true for 500, 502, 503, 429, and non-nil network error
  - [ ] `isRetryable` returns false for 200, 201, 400, 401, 404

### Phase 4 — Auth (`auth.go`)
- [ ] Define unexported `tokenManager` struct with fields: `mu sync.Mutex`, `httpClient`, `baseURL`, `clientID`, `clientSecret`, `logger`, `token string`, `expiresAt time.Time`
- [ ] Implement `(*tokenManager).Token(ctx context.Context) (string, error)` — fast path returns cached token if `time.Now().Before(expiresAt - 60s)`
- [ ] Implement unexported `(*tokenManager).refresh(ctx context.Context) (string, error)` — called only while holding `mu`
  - [ ] Build the JSON request body with `grantType: "client_credentials"`, `clientId`, `clientSecret`, `code: ""`
  - [ ] POST to `/oauth/v1/accesstokens` using the raw `httpClient` (not `do()`, to avoid circular dependency)
  - [ ] Decode response into `accessTokenResponse`
  - [ ] Parse `ExpiresIn` string → int with `strconv.Atoi`
  - [ ] Parse `IssuedAt` string → `int64` with `strconv.ParseInt`, then `time.UnixMilli`
  - [ ] Store `token` and computed `expiresAt` on the manager
  - [ ] Log the refresh event at `DEBUG` level via `slog`
  - [ ] Wrap and return any parse or HTTP error with `"indigo: auth: "` prefix
- [ ] Write unit tests in `auth_test.go` using `httptest.NewServer`:
  - [ ] Successful token fetch populates `token` and `expiresAt`
  - [ ] Second `Token()` call within TTL returns cached token (assert server called only once)
  - [ ] `Token()` call after expiry triggers a second server call
  - [ ] Server returning non-201 produces an `*APIError`
  - [ ] Malformed `expiresIn` / `issuedAt` returns a wrapped parse error
  - [ ] Concurrent `Token()` calls do not result in duplicate fetches (run with `-race`)

### Phase 5 — HTTP transport (`http.go`)
- [ ] Add `addQueryInt(q url.Values, key string, val int)` helper — appends only if `val > 0`
- [ ] Implement unexported `(*Client).doRaw(ctx, req, authenticated bool) (*http.Response, error)`:
  - [ ] If `authenticated`, call `c.token.Token(ctx)` and set `Authorization: Bearer <token>` header
  - [ ] Set `Content-Type: application/json` and `Accept: application/json` headers
  - [ ] Execute via `c.httpClient.Do(req)`
  - [ ] Log outgoing request at `DEBUG`: method, path, attempt
  - [ ] Log response at `DEBUG`: status code, elapsed duration
- [ ] Implement `(*Client).do(ctx, method, path string, body any, dst any, authenticated bool) error`:
  - [ ] Serialize `body` to `[]byte` once before the retry loop (handle `nil` body)
  - [ ] Retry loop up to `c.retry.MaxAttempts`:
    - [ ] Construct `*http.Request` with `bytes.NewReader(bodyBytes)` and correct `Content-Length`
    - [ ] Call `doRaw`
    - [ ] On success (2xx): decode response body into `dst` if `dst != nil`; return `nil`
    - [ ] On non-2xx: read body, construct `*APIError`; if retryable and attempts remain, wait `retry.delay(attempt)` with context-aware sleep; otherwise return the error
    - [ ] On network error: same retry/wait logic
    - [ ] Log retry event at `INFO` with attempt number, status, and backoff duration
  - [ ] Context-aware sleep between retries: `select { case <-ctx.Done(): ... case <-time.After(wait): }`
- [ ] Write unit tests in `http_test.go`:
  - [ ] 2xx response decodes JSON into `dst` correctly
  - [ ] 500 response is retried up to `MaxAttempts`; assert server call count
  - [ ] 429 response is retried; assert backoff wait is respected (use a fast `RetryConfig` with tiny delays)
  - [ ] 400 response is NOT retried; returned immediately as `*APIError`
  - [ ] Context cancellation during backoff sleep is respected
  - [ ] `Authorization` header is present on authenticated calls
  - [ ] `Authorization` header is absent on unauthenticated calls (token endpoint)

### Phase 6 — Client entrypoint (`indigo.go`)
- [ ] Define `Client` struct with unexported fields (`baseURL`, `httpClient`, `token`, `retry`, `logger`, `rng`) and public service fields (`SSH SSHKeyService`, `Instance InstanceService`)
- [ ] Define `Option` as `func(*Client)`
- [ ] Implement `WithBaseURL(u string) Option`
- [ ] Implement `WithHTTPClient(hc *http.Client) Option`
- [ ] Implement `WithLogger(l *slog.Logger) Option`
- [ ] Implement `WithRetryConfig(r RetryConfig) Option`
- [ ] Implement `WithTimeout(d time.Duration) Option` — sets `Timeout` on the default `http.Client`; no-op if a custom client was provided
- [ ] Implement `NewClient(clientID, clientSecret string, opts ...Option) *Client`:
  - [ ] Initialize with defaults (30 s timeout, `slog.Default()`, `DefaultRetryConfig()`, production base URL)
  - [ ] Apply all options
  - [ ] Construct `tokenManager` and assign to `c.token`
  - [ ] Assign `c.SSH` and `c.Instance` with reference to `c`
  - [ ] Seed internal `rand.Rand` for jitter (store on `Client` or pass to `RetryConfig`)
- [ ] Add package doc comment (`// Package indigo provides a Go client for the WebARENA Indigo VPS API.`)
- [ ] Write unit tests in `indigo_test.go`:
  - [ ] `NewClient` with no options sets expected defaults
  - [ ] `WithBaseURL` overrides the URL
  - [ ] `WithTimeout` affects the default `http.Client`
  - [ ] `WithHTTPClient` replaces the default client

### Phase 7 — SSH Key service (`sshkey.go`)
- [ ] Implement `SSHKeyService.List(ctx) ([]SSHKey, error)` — GET `/webarenaIndigo/v1/vm/sshkey`
- [ ] Implement `SSHKeyService.ListActive(ctx) ([]SSHKey, error)` — GET `/webarenaIndigo/v1/vm/sshkey/active/status`
- [ ] Implement `SSHKeyService.Get(ctx, id int) (*SSHKey, error)` — GET `/webarenaIndigo/v1/vm/sshkey/{id}`; spec returns an array — decode as `[]SSHKey`, return index 0; return `*APIError` with 404 if array is empty
- [ ] Implement `SSHKeyService.Create(ctx, req CreateSSHKeyRequest) (*SSHKey, error)` — POST `/webarenaIndigo/v1/vm/sshkey`
- [ ] Implement `SSHKeyService.Update(ctx, id int, req UpdateSSHKeyRequest) error` — PUT `/webarenaIndigo/v1/vm/sshkey/{id}`; return only error (response carries no data beyond `success`/`message`)
- [ ] Implement `SSHKeyService.Delete(ctx, id int) error` — DELETE `/webarenaIndigo/v1/vm/sshkey/{id}`
- [ ] Write unit tests in `sshkey_test.go` using `testutil.MockServer`:
  - [ ] `List` returns slice of `SSHKey`
  - [ ] `ListActive` returns only active keys
  - [ ] `Get` returns the correct key; array-to-single-object unwrapping works
  - [ ] `Get` on empty array returns a not-found error (verify `IsNotFound`)
  - [ ] `Create` sends correct JSON body; returns created key
  - [ ] `Update` sends correct JSON body; no error on success
  - [ ] `Delete` issues DELETE to correct path; no error on success

### Phase 8 — Instance service (`instance.go`)
- [ ] Implement `InstanceService.ListTypes(ctx) ([]InstanceType, error)` — GET `/webarenaIndigo/v1/vm/instancetypes`
- [ ] Implement `InstanceService.ListRegions(ctx, instanceTypeID int) ([]Region, error)` — GET `/webarenaIndigo/v1/vm/getregion`; use `addQueryInt` for `instanceTypeID`
- [ ] Implement `InstanceService.ListOS(ctx, instanceTypeID int) ([]OSCategory, error)` — GET `/webarenaIndigo/v1/vm/oslist`; spec leaves `osCategory` items untyped — use `json.RawMessage` or a minimal struct; `OSCategory` type to be defined in `models.go`
- [ ] Implement `InstanceService.ListSpecs(ctx, instanceTypeID, osID int) ([]InstanceSpec, error)` — GET `/webarenaIndigo/v1/vm/getinstancespec`; use `addQueryInt` for both params
- [ ] Implement `InstanceService.Create(ctx, req CreateInstanceRequest) (*Instance, error)` — POST `/webarenaIndigo/v1/vm/createinstance`; decode `vms` field from envelope
- [ ] Implement `InstanceService.List(ctx) ([]Instance, error)` — GET `/webarenaIndigo/v1/vm/getinstancelist`; spec returns a top-level array (no envelope)
- [ ] Implement `InstanceService.UpdateStatus(ctx, instanceID string, action InstanceAction) (*UpdateInstanceStatusResult, error)` — POST `/webarenaIndigo/v1/vm/instance/statusupdate`
- [ ] Implement convenience wrappers `Start`, `Stop`, `ForceStop`, `Reset`, `Destroy` — each delegates to `UpdateStatus` with the appropriate `InstanceAction` constant
- [ ] Write unit tests in `instance_test.go` using `testutil.MockServer`:
  - [ ] `ListTypes` parses `instanceTypes` array
  - [ ] `ListRegions` with non-zero `instanceTypeID` sends query param; with `0` omits it
  - [ ] `ListOS` with non-zero `instanceTypeID` sends query param; with `0` omits it
  - [ ] `ListSpecs` with both params sends both; with zero values omits them
  - [ ] `Create` sends correct body; returns `Instance` from `vms` field
  - [ ] `List` parses top-level array response (no envelope)
  - [ ] `UpdateStatus` sends correct `instanceId` and `status` string
  - [ ] Each convenience wrapper (`Start`, `Stop`, etc.) sends the correct `status` string
  - [ ] `UpdateInstanceStatusResult.InstanceStatus` is populated from the response

### Phase 9 — Test infrastructure (`internal/testutil/mock_server.go`)
- [ ] Define `MockServer` struct holding `*httptest.Server`, a slice of recorded `*http.Request`, and a `sync.Mutex` for the slice
- [ ] Implement `NewMockServer(handlers map[string]http.HandlerFunc) *MockServer` — keys are `"METHOD /path"` strings; unmatched requests respond 404; all requests are appended to the recorded slice
- [ ] Implement `(*MockServer).NewClient(opts ...Option) *Client` — sets `WithBaseURL` to the test server URL; pre-injects a dummy token so tests bypass the auth endpoint by default
- [ ] Implement `(*MockServer).Close()` delegating to `httptest.Server.Close()`
- [ ] Implement `(*MockServer).RequestCount(method, path string) int` helper for asserting call counts in tests
- [ ] Implement `(*MockServer).LastRequest(method, path string) *http.Request` helper for inspecting the last request body/headers

### Phase 10 — README (`README.md`)
- [ ] Write package overview and installation section (`go get`)
- [ ] Document authentication (clientId / clientSecret; token auto-managed)
- [ ] Write quick-start example: create client, list instances, start one
- [ ] Document all `Option` constructors with usage examples
- [ ] Document `RetryConfig` and how to override defaults
- [ ] Document structured logging integration (`WithLogger`, `slog.New(...)`)
- [ ] Document error handling pattern (`errors.As`, `IsNotFound`, `IsUnauthorized`)
- [ ] Document `context.Context` usage and cancellation
- [ ] Add a section on running tests (`go test -race ./...`)

---

## Spec Quirk Handling Summary

| Quirk | Location | Handling |
|---|---|---|
| `expiresIn` is a string `"3599"` | `auth.go` | `strconv.Atoi` after decode |
| `issuedAt` is Unix-ms string `"1550570350202"` | `auth.go` | `strconv.ParseInt` + `time.UnixMilli` |
| Optional query params omitted when 0 | `http.go` | `addQueryInt` helper |
| Instance status enum has YAML formatting error | `models.go` | Hardcoded constants; spec not trusted |
| `instanceStatus` in status-update response | `models.go` | Exposed in `UpdateInstanceStatusResult` |
| Get SSH Key returns `sshKey` as array, not object | `sshkey.go` | Decode as `[]SSHKey`, return first element |
