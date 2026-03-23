package indigo_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	indigo "github.com/barn0w1/indigo-webarena-go"
	"github.com/barn0w1/indigo-webarena-go/internal/testutil"
)

func newSSHTestClient(ms *testutil.MockServer) *indigo.Client {
	return indigo.NewClient("id", "secret",
		indigo.WithBaseURL(ms.Server.URL),
		indigo.WithRetryConfig(indigo.RetryConfig{
			MaxAttempts: 1,
			BaseDelay:   time.Millisecond,
			MaxDelay:    time.Millisecond,
			Multiplier:  1,
		}),
	)
}

func sampleKey(id int, name, status string) map[string]interface{} {
	return map[string]interface{}{
		"id":         id,
		"service_id": "svc1",
		"user_id":    42,
		"name":       name,
		"sshkey":     "ssh-rsa AAAA...",
		"status":     status,
		"created_at": "2024-01-01T00:00:00Z",
		"updated_at": "2024-01-01T00:00:00Z",
	}
}

func TestSSHKeyList(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/sshkey": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true,
				"total":   2,
				"sshkeys": []interface{}{sampleKey(1, "key-a", "ACTIVE"), sampleKey(2, "key-b", "DEACTIVE")},
			})
		},
	})
	defer ms.Close()

	keys, err := newSSHTestClient(ms).SSH.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2", len(keys))
	}
	if keys[0].Name != "key-a" {
		t.Errorf("keys[0].Name = %q, want %q", keys[0].Name, "key-a")
	}
}

func TestSSHKeyListActive(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/sshkey/active/status": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true,
				"total":   1,
				"sshkeys": []interface{}{sampleKey(1, "key-a", "ACTIVE")},
			})
		},
	})
	defer ms.Close()

	keys, err := newSSHTestClient(ms).SSH.ListActive(context.Background())
	if err != nil {
		t.Fatalf("ListActive() error: %v", err)
	}
	if len(keys) != 1 || keys[0].Status != indigo.SSHKeyStatusActive {
		t.Errorf("unexpected keys: %+v", keys)
	}
}

func TestSSHKeyGet(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/sshkey/7": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true,
				"sshKey":  []interface{}{sampleKey(7, "my-key", "ACTIVE")},
			})
		},
	})
	defer ms.Close()

	key, err := newSSHTestClient(ms).SSH.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if key.ID != 7 || key.Name != "my-key" {
		t.Errorf("unexpected key: %+v", key)
	}
}

func TestSSHKeyGetEmptyArrayIsNotFound(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/sshkey/99": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true,
				"sshKey":  []interface{}{},
			})
		},
	})
	defer ms.Close()

	_, err := newSSHTestClient(ms).SSH.Get(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error for empty array")
	}
	if !indigo.IsNotFound(err) {
		t.Errorf("expected IsNotFound, got: %v", err)
	}
}

func TestSSHKeyCreate(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"POST /webarenaIndigo/v1/vm/sshkey": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true,
				"message": "created",
				"sshKey":  sampleKey(10, body["sshName"], "ACTIVE"),
			})
		},
	})
	defer ms.Close()

	key, err := newSSHTestClient(ms).SSH.Create(context.Background(), indigo.CreateSSHKeyRequest{
		Name:      "new-key",
		PublicKey: "ssh-rsa BBBB...",
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if key.Name != "new-key" {
		t.Errorf("key.Name = %q, want %q", key.Name, "new-key")
	}
}

func TestSSHKeyUpdate(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"PUT /webarenaIndigo/v1/vm/sshkey/5": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
			if body["sshName"] != "renamed" {
				http.Error(w, "wrong name", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "updated"}) //nolint:errcheck
		},
	})
	defer ms.Close()

	err := newSSHTestClient(ms).SSH.Update(context.Background(), 5, indigo.UpdateSSHKeyRequest{Name: "renamed"})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
}

func TestSSHKeyDelete(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"DELETE /webarenaIndigo/v1/vm/sshkey/3": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "deleted"}) //nolint:errcheck
		},
	})
	defer ms.Close()

	err := newSSHTestClient(ms).SSH.Delete(context.Background(), 3)
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
}

func TestSSHKeyAPIError(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/sshkey": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "server error", http.StatusInternalServerError)
		},
	})
	defer ms.Close()

	_, err := newSSHTestClient(ms).SSH.List(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *indigo.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusInternalServerError)
	}
}
