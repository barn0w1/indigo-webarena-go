package indigo

import (
	"context"
	"net/http"
	"net/url"
)

// InstanceService wraps all instance endpoints.
type InstanceService struct {
	client *Client
}

// ListTypes returns all available instance types.
func (s InstanceService) ListTypes(ctx context.Context) ([]InstanceType, error) {
	var envelope struct {
		Success       bool           `json:"success"`
		Total         int            `json:"total"`
		InstanceTypes []InstanceType `json:"instanceTypes"`
	}
	if err := s.client.do(ctx, http.MethodGet, "/webarenaIndigo/v1/vm/instancetypes", nil, &envelope, true); err != nil {
		return nil, err
	}
	return envelope.InstanceTypes, nil
}

// ListRegions returns available regions. Pass instanceTypeID=0 to list all regions.
func (s InstanceService) ListRegions(ctx context.Context, instanceTypeID int) ([]Region, error) {
	q := url.Values{}
	addQueryInt(q, "instanceTypeId", instanceTypeID)
	path := "/webarenaIndigo/v1/vm/getregion"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var envelope struct {
		Success    bool     `json:"success"`
		Total      int      `json:"total"`
		RegionList []Region `json:"regionlist"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, nil, &envelope, true); err != nil {
		return nil, err
	}
	return envelope.RegionList, nil
}

// ListOS returns available operating systems. Pass instanceTypeID=0 for all.
func (s InstanceService) ListOS(ctx context.Context, instanceTypeID int) ([]OSCategory, error) {
	q := url.Values{}
	addQueryInt(q, "instanceTypeId", instanceTypeID)
	path := "/webarenaIndigo/v1/vm/oslist"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var envelope struct {
		Success    bool         `json:"success"`
		Total      int          `json:"total"`
		OSCategory []OSCategory `json:"osCategory"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, nil, &envelope, true); err != nil {
		return nil, err
	}
	return envelope.OSCategory, nil
}

// ListSpecs returns available instance specifications.
// Pass zero values to omit the corresponding query parameter.
func (s InstanceService) ListSpecs(ctx context.Context, instanceTypeID, osID int) ([]InstanceSpec, error) {
	q := url.Values{}
	addQueryInt(q, "instanceTypeId", instanceTypeID)
	addQueryInt(q, "osId", osID)
	path := "/webarenaIndigo/v1/vm/getinstancespec"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var envelope struct {
		Success  bool           `json:"success"`
		Total    int            `json:"total"`
		SpecList []InstanceSpec `json:"speclist"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, nil, &envelope, true); err != nil {
		return nil, err
	}
	return envelope.SpecList, nil
}

// Create provisions a new instance.
func (s InstanceService) Create(ctx context.Context, req CreateInstanceRequest) (*Instance, error) {
	var envelope struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		VMs     Instance `json:"vms"`
	}
	if err := s.client.do(ctx, http.MethodPost, "/webarenaIndigo/v1/vm/createinstance", req, &envelope, true); err != nil {
		return nil, err
	}
	return &envelope.VMs, nil
}

// List returns all instances in the account.
func (s InstanceService) List(ctx context.Context) ([]Instance, error) {
	var instances []Instance
	if err := s.client.do(ctx, http.MethodGet, "/webarenaIndigo/v1/vm/getinstancelist", nil, &instances, true); err != nil {
		return nil, err
	}
	return instances, nil
}

// UpdateStatus applies an action to an instance.
func (s InstanceService) UpdateStatus(ctx context.Context, instanceID string, action InstanceAction) (*UpdateInstanceStatusResult, error) {
	body := struct {
		InstanceID string         `json:"instanceId"`
		Status     InstanceAction `json:"status"`
	}{
		InstanceID: instanceID,
		Status:     action,
	}
	var result UpdateInstanceStatusResult
	if err := s.client.do(ctx, http.MethodPost, "/webarenaIndigo/v1/vm/instance/statusupdate", body, &result, true); err != nil {
		return nil, err
	}
	return &result, nil
}

// Start starts a stopped instance.
func (s InstanceService) Start(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error) {
	return s.UpdateStatus(ctx, instanceID, InstanceActionStart)
}

// Stop gracefully stops a running instance.
func (s InstanceService) Stop(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error) {
	return s.UpdateStatus(ctx, instanceID, InstanceActionStop)
}

// ForceStop forcefully halts a running instance.
func (s InstanceService) ForceStop(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error) {
	return s.UpdateStatus(ctx, instanceID, InstanceActionForceStop)
}

// Reset reboots an instance.
func (s InstanceService) Reset(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error) {
	return s.UpdateStatus(ctx, instanceID, InstanceActionReset)
}

// Destroy permanently deletes an instance.
func (s InstanceService) Destroy(ctx context.Context, instanceID string) (*UpdateInstanceStatusResult, error) {
	return s.UpdateStatus(ctx, instanceID, InstanceActionDestroy)
}
