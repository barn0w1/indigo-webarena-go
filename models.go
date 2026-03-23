package indigo

import (
	"encoding/json"
	"time"
)

// SSHKeyStatus represents the activation state of an SSH key.
type SSHKeyStatus string

const (
	SSHKeyStatusActive   SSHKeyStatus = "ACTIVE"
	SSHKeyStatusDeactive SSHKeyStatus = "DEACTIVE"
)

// InstanceAction is the operation to apply to an instance.
type InstanceAction string

const (
	InstanceActionStart     InstanceAction = "start"
	InstanceActionStop      InstanceAction = "stop"
	InstanceActionForceStop InstanceAction = "forcestop"
	InstanceActionReset     InstanceAction = "reset"
	InstanceActionDestroy   InstanceAction = "destroy"
)

// accessTokenResponse is the raw JSON shape returned by POST /oauth/v1/accesstokens.
// ExpiresIn and IssuedAt are strings per the spec quirk.
type accessTokenResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   string `json:"expiresIn"`
	Scope       string `json:"scope"`
	IssuedAt    string `json:"issuedAt"`
}

// SSHKey represents a single SSH public key in the account.
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

// InstanceType describes an available instance type.
type InstanceType struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// Region describes an available deployment region.
type Region struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// OSCategory holds the raw OS category data returned by the list-OS endpoint.
// The spec does not define the item schema, so the raw JSON is preserved.
type OSCategory struct {
	Raw json.RawMessage
}

func (o *OSCategory) UnmarshalJSON(data []byte) error {
	o.Raw = append([]byte(nil), data...)
	return nil
}

// InstanceSpec describes a single available plan/specification.
type InstanceSpec struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Instance represents a VPS instance in the account.
type Instance struct {
	ID       int    `json:"id"`
	Name     string `json:"instance_name"`
	Status   string `json:"status"`
	SSHKeyID int    `json:"sshkey_id"`
	HostID   int    `json:"host_id"`
	Plan     string `json:"plan"`
	MemSize  int    `json:"memsize"`
	CPUs     int    `json:"cpus"`
	OSID     int    `json:"os_id"`
	UUID     string `json:"uuid"`
	IP       string `json:"ip"`
	ArpaName string `json:"arpaname"`
}

// UpdateInstanceStatusResult is the response from the status-update endpoint.
type UpdateInstanceStatusResult struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	SuccessCode    string `json:"successCode"`
	InstanceStatus string `json:"instanceStatus"`
}

// CreateSSHKeyRequest is the body for creating a new SSH key.
type CreateSSHKeyRequest struct {
	Name      string `json:"sshName"`
	PublicKey string `json:"sshKey"`
}

// UpdateSSHKeyRequest is the body for updating an existing SSH key.
// Zero-value fields are omitted from the request.
type UpdateSSHKeyRequest struct {
	Name      string       `json:"sshName,omitempty"`
	PublicKey string       `json:"sshKey,omitempty"`
	Status    SSHKeyStatus `json:"sshKeyStatus,omitempty"`
}

// CreateInstanceRequest is the body for creating a new instance.
// Pointer fields are omitted when nil so zero-value integers are never sent.
type CreateInstanceRequest struct {
	Name        string `json:"instanceName"`
	Plan        int    `json:"instancePlan"`
	RegionID    *int   `json:"regionId,omitempty"`
	OSID        *int   `json:"osId,omitempty"`
	SSHKeyID    *int   `json:"sshKeyId,omitempty"`
	WinPassword string `json:"winPassword,omitempty"`
	ImportURL   string `json:"importUrl,omitempty"`
	SnapshotID  *int   `json:"snapshotId,omitempty"`
}
