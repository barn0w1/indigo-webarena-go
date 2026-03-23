# indigo-webarena-go

A production-quality Go client SDK for the [WebARENA Indigo](https://web.arena.ne.jp/indigo/) VPS API, operated by NTT.

> ⚠️ This is an unofficial SDK and is not affiliated with or endorsed by NTT or WebARENA.

Zero third-party runtime dependencies. Goroutine-safe. Context-aware throughout.

## Installation

```
go get github.com/barn0w1/indigo-webarena-go
```

## Authentication

Obtain your **API Key** (`clientId`) and **API Private Key** (`clientSecret`) from the WebARENA Indigo control panel.

The SDK manages tokens automatically: it fetches a token on the first API call, caches it, and refreshes it 60 seconds before expiry. You never handle tokens directly.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    indigo "github.com/barn0w1/indigo-webarena-go"
)

func main() {
    client := indigo.NewClient("YOUR_CLIENT_ID", "YOUR_CLIENT_SECRET")

    ctx := context.Background()

    instances, err := client.Instance.List(ctx)
    if err != nil {
        log.Fatal(err)
    }
    for _, inst := range instances {
        fmt.Printf("%d  %-20s  %s  %s\n", inst.ID, inst.Name, inst.Status, inst.IP)
    }

    // Start an instance
    result, err := client.Instance.Start(ctx, "your-instance-id")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Status:", result.InstanceStatus)
}
```

## Client options

```go
client := indigo.NewClient(clientID, clientSecret,
    // Override the API base URL (useful for testing or proxies)
    indigo.WithBaseURL("https://api.customer.jp"),

    // Supply a custom *http.Client (e.g. with a custom transport or proxy)
    indigo.WithHTTPClient(myHTTPClient),

    // Set the timeout on the default HTTP client (default: 30s)
    indigo.WithTimeout(15 * time.Second),

    // Attach a structured logger
    indigo.WithLogger(slog.Default()),

    // Override retry behaviour
    indigo.WithRetryConfig(indigo.RetryConfig{
        MaxAttempts: 5,
        BaseDelay:   250 * time.Millisecond,
        MaxDelay:    30 * time.Second,
        Multiplier:  2.0,
    }),
)
```

## Retry behaviour

By default the client retries up to **3 times** on:

- Network-level errors (timeouts, connection resets)
- HTTP `429 Too Many Requests`
- HTTP `5xx` server errors

Retries use exponential backoff (base 500 ms, max 30 s, 2x multiplier) with +/-25% jitter. Use `WithRetryConfig` to tune or disable retries (`MaxAttempts: 1`).

## Structured logging

The client logs at `DEBUG` for each request/response and at `INFO` for retries. Pass any `*slog.Logger`:

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
client := indigo.NewClient(clientID, clientSecret, indigo.WithLogger(logger))
```

## Error handling

Non-2xx responses are returned as `*APIError`:

```go
_, err := client.SSH.Get(ctx, 999)
if err != nil {
    var apiErr *indigo.APIError
    if errors.As(err, &apiErr) {
        fmt.Println("HTTP status:", apiErr.StatusCode)
        fmt.Println("Body:", apiErr.Body)
    }
}
```

Convenience predicates:

```go
if indigo.IsNotFound(err) { ... }
if indigo.IsUnauthorized(err) { ... }
```

Context errors propagate normally:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

_, err := client.Instance.List(ctx)
if errors.Is(err, context.DeadlineExceeded) { ... }
```

## SSH Key management

```go
// List all / active only
keys, err := client.SSH.List(ctx)
keys, err := client.SSH.ListActive(ctx)

// Get by ID
key, err := client.SSH.Get(ctx, 42)

// Create
key, err := client.SSH.Create(ctx, indigo.CreateSSHKeyRequest{
    Name:      "my-laptop",
    PublicKey: "ssh-rsa AAAA...",
})

// Update (all fields optional)
err = client.SSH.Update(ctx, 42, indigo.UpdateSSHKeyRequest{
    Name:   "new-name",
    Status: indigo.SSHKeyStatusDeactive,
})

// Delete
err = client.SSH.Delete(ctx, 42)
```

## Instance management

```go
// Lookup available options
types, err   := client.Instance.ListTypes(ctx)
regions, err := client.Instance.ListRegions(ctx, instanceTypeID) // 0 = all
osList, err  := client.Instance.ListOS(ctx, instanceTypeID)      // 0 = all
specs, err   := client.Instance.ListSpecs(ctx, instanceTypeID, osID) // 0 = all

// Create
sshKeyID := 7
inst, err := client.Instance.Create(ctx, indigo.CreateInstanceRequest{
    Name:     "web-01",
    Plan:     3,
    RegionID: &regionID,
    OSID:     &osID,
    SSHKeyID: &sshKeyID,
})

// List all instances
instances, err := client.Instance.List(ctx)

// Status transitions
result, err := client.Instance.Start(ctx, instanceID)
result, err := client.Instance.Stop(ctx, instanceID)
result, err := client.Instance.ForceStop(ctx, instanceID)
result, err := client.Instance.Reset(ctx, instanceID)
result, err := client.Instance.Destroy(ctx, instanceID)

// Or use UpdateStatus directly
result, err := client.Instance.UpdateStatus(ctx, instanceID, indigo.InstanceActionStart)
```

Optional integer fields on `CreateInstanceRequest` (`RegionID`, `OSID`, `SSHKeyID`, `SnapshotID`) are pointer types. Pass `nil` to omit them from the request entirely.

## Running tests

```
go test -race ./...
```