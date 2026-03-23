package indigo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	indigo "github.com/barn0w1/indigo-webarena-go"
	"github.com/barn0w1/indigo-webarena-go/internal/testutil"
)

func newInstTestClient(ms *testutil.MockServer) *indigo.Client {
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

func sampleInstance(id int, name string) map[string]interface{} {
	return map[string]interface{}{
		"id":            id,
		"instance_name": name,
		"status":        "running",
		"sshkey_id":     1,
		"host_id":       2,
		"plan":          "standard",
		"memsize":       2048,
		"cpus":          2,
		"os_id":         10,
		"uuid":          "abc-123",
		"ip":            "1.2.3.4",
		"arpaname":      "4.3.2.1.in-addr.arpa",
	}
}

func TestInstanceListTypes(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/instancetypes": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true,
				"total":   1,
				"instanceTypes": []interface{}{
					map[string]interface{}{"id": 1, "name": "vps", "display_name": "VPS"},
				},
			})
		},
	})
	defer ms.Close()

	types, err := newInstTestClient(ms).Instance.ListTypes(context.Background())
	if err != nil {
		t.Fatalf("ListTypes() error: %v", err)
	}
	if len(types) != 1 || types[0].Name != "vps" {
		t.Errorf("unexpected types: %+v", types)
	}
}

func TestInstanceListRegionsWithParam(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/getregion": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query().Get("instanceTypeId")
			if q != "5" {
				http.Error(w, "missing param", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success":    true,
				"total":      1,
				"regionlist": []interface{}{map[string]interface{}{"id": 1, "name": "tokyo"}},
			})
		},
	})
	defer ms.Close()

	regions, err := newInstTestClient(ms).Instance.ListRegions(context.Background(), 5)
	if err != nil {
		t.Fatalf("ListRegions() error: %v", err)
	}
	if len(regions) != 1 || regions[0].Name != "tokyo" {
		t.Errorf("unexpected regions: %+v", regions)
	}
}

func TestInstanceListRegionsNoParam(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/getregion": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("instanceTypeId") {
				http.Error(w, "unexpected param", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true, "total": 0, "regionlist": []interface{}{},
			})
		},
	})
	defer ms.Close()

	_, err := newInstTestClient(ms).Instance.ListRegions(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListRegions(0) error: %v", err)
	}
}

func TestInstanceListOS(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/oslist": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success":    true,
				"total":      1,
				"osCategory": []interface{}{map[string]interface{}{"id": 1, "name": "Ubuntu"}},
			})
		},
	})
	defer ms.Close()

	os, err := newInstTestClient(ms).Instance.ListOS(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListOS() error: %v", err)
	}
	if len(os) != 1 {
		t.Errorf("got %d os items, want 1", len(os))
	}
}

func TestInstanceListSpecs(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/getinstancespec": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("instanceTypeId") != "2" || q.Get("osId") != "10" {
				http.Error(w, "wrong params", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true,
				"total":   1,
				"speclist": []interface{}{
					map[string]interface{}{"id": 3, "name": "2core-2gb", "description": "2 CPU, 2GB RAM"},
				},
			})
		},
	})
	defer ms.Close()

	specs, err := newInstTestClient(ms).Instance.ListSpecs(context.Background(), 2, 10)
	if err != nil {
		t.Fatalf("ListSpecs() error: %v", err)
	}
	if len(specs) != 1 || specs[0].Name != "2core-2gb" {
		t.Errorf("unexpected specs: %+v", specs)
	}
}

func TestInstanceListSpecsNoParams(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/getinstancespec": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Has("instanceTypeId") || q.Has("osId") {
				http.Error(w, "unexpected params", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true, "total": 0, "speclist": []interface{}{},
			})
		},
	})
	defer ms.Close()

	_, err := newInstTestClient(ms).Instance.ListSpecs(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("ListSpecs(0,0) error: %v", err)
	}
}

func TestInstanceCreate(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"POST /webarenaIndigo/v1/vm/createinstance": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true,
				"message": "created",
				"vms":     sampleInstance(42, body["instanceName"].(string)),
			})
		},
	})
	defer ms.Close()

	inst, err := newInstTestClient(ms).Instance.Create(context.Background(), indigo.CreateInstanceRequest{
		Name: "my-server",
		Plan: 3,
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if inst.Name != "my-server" || inst.ID != 42 {
		t.Errorf("unexpected instance: %+v", inst)
	}
}

func TestInstanceList(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"GET /webarenaIndigo/v1/vm/getinstancelist": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]interface{}{ //nolint:errcheck
				sampleInstance(1, "srv-1"),
				sampleInstance(2, "srv-2"),
			})
		},
	})
	defer ms.Close()

	instances, err := newInstTestClient(ms).Instance.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("got %d instances, want 2", len(instances))
	}
}

func TestInstanceUpdateStatus(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"POST /webarenaIndigo/v1/vm/instance/statusupdate": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success":        true,
				"message":        "ok",
				"successCode":    "200",
				"instanceStatus": body["status"],
			})
		},
	})
	defer ms.Close()

	result, err := newInstTestClient(ms).Instance.UpdateStatus(context.Background(), "inst-abc", indigo.InstanceActionStop)
	if err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}
	if result.InstanceStatus != string(indigo.InstanceActionStop) {
		t.Errorf("InstanceStatus = %q, want %q", result.InstanceStatus, string(indigo.InstanceActionStop))
	}
}


func TestInstanceConvenienceWrappers(t *testing.T) {
	actions := []struct {
		name   string
		action indigo.InstanceAction
	}{
		{"start", indigo.InstanceActionStart},
		{"stop", indigo.InstanceActionStop},
		{"forcestop", indigo.InstanceActionForceStop},
		{"reset", indigo.InstanceActionReset},
		{"destroy", indigo.InstanceActionDestroy},
	}

	for _, tt := range actions {
		t.Run(tt.name, func(t *testing.T) {
			ms := testutil.NewMockServer(map[string]http.HandlerFunc{
				"POST /webarenaIndigo/v1/vm/instance/statusupdate": func(w http.ResponseWriter, r *http.Request) {
					var body map[string]string
					json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
						"success":        true,
						"instanceStatus": body["status"],
					})
				},
			})
			defer ms.Close()

			c := newInstTestClient(ms)
			var result *indigo.UpdateInstanceStatusResult
			var err error

			switch tt.action {
			case indigo.InstanceActionStart:
				result, err = c.Instance.Start(context.Background(), "inst-1")
			case indigo.InstanceActionStop:
				result, err = c.Instance.Stop(context.Background(), "inst-1")
			case indigo.InstanceActionForceStop:
				result, err = c.Instance.ForceStop(context.Background(), "inst-1")
			case indigo.InstanceActionReset:
				result, err = c.Instance.Reset(context.Background(), "inst-1")
			case indigo.InstanceActionDestroy:
				result, err = c.Instance.Destroy(context.Background(), "inst-1")
			}

			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if result.InstanceStatus != string(tt.action) {
				t.Errorf("InstanceStatus = %q, want %q", result.InstanceStatus, string(tt.action))
			}
		})
	}
}

func TestInstanceCreateOptionalFieldsOmitted(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"POST /webarenaIndigo/v1/vm/createinstance": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
			// regionId, osId, sshKeyId, snapshotId must NOT appear in the body.
			for _, key := range []string{"regionId", "osId", "sshKeyId", "snapshotId"} {
				if _, ok := body[key]; ok {
					http.Error(w, "unexpected field: "+key, http.StatusBadRequest)
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true, "vms": sampleInstance(1, "test"),
			})
		},
	})
	defer ms.Close()

	_, err := newInstTestClient(ms).Instance.Create(context.Background(), indigo.CreateInstanceRequest{
		Name: "test",
		Plan: 1,
	})
	if err != nil {
		// surface the server error message if any
		t.Fatalf("Create() error: %v — check that optional fields are omitted", err)
	}
}

func TestInstanceUpdateStatusSendsCorrectBody(t *testing.T) {
	ms := testutil.NewMockServer(map[string]http.HandlerFunc{
		"POST /webarenaIndigo/v1/vm/instance/statusupdate": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
			if body["instanceId"] != "vm-xyz" {
				http.Error(w, "wrong instanceId: "+body["instanceId"], http.StatusBadRequest)
				return
			}
			if !strings.Contains("start stop forcestop reset destroy", body["status"]) {
				http.Error(w, "invalid status: "+body["status"], http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"success": true, "instanceStatus": body["status"],
			})
		},
	})
	defer ms.Close()

	result, err := newInstTestClient(ms).Instance.Start(context.Background(), "vm-xyz")
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if result.InstanceStatus != "start" {
		t.Errorf("InstanceStatus = %q, want start", result.InstanceStatus)
	}
}
