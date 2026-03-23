package indigo

import (
	"context"
	"fmt"
	"net/http"
)

// SSHKeyService wraps all SSH key endpoints.
type SSHKeyService struct {
	client *Client
}

// List returns all SSH keys in the account.
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

// ListActive returns all SSH keys with ACTIVE status.
func (s SSHKeyService) ListActive(ctx context.Context) ([]SSHKey, error) {
	var envelope struct {
		Success bool     `json:"success"`
		Total   int      `json:"total"`
		SSHKeys []SSHKey `json:"sshkeys"`
	}
	if err := s.client.do(ctx, http.MethodGet, "/webarenaIndigo/v1/vm/sshkey/active/status", nil, &envelope, true); err != nil {
		return nil, err
	}
	return envelope.SSHKeys, nil
}

// Get returns the SSH key with the given ID.
// The spec returns the key wrapped in an array; this method unwraps it.
func (s SSHKeyService) Get(ctx context.Context, id int) (*SSHKey, error) {
	var envelope struct {
		Success bool     `json:"success"`
		SSHKey  []SSHKey `json:"sshKey"`
	}
	path := fmt.Sprintf("/webarenaIndigo/v1/vm/sshkey/%d", id)
	if err := s.client.do(ctx, http.MethodGet, path, nil, &envelope, true); err != nil {
		return nil, err
	}
	if len(envelope.SSHKey) == 0 {
		return nil, &APIError{StatusCode: http.StatusNotFound, Body: "ssh key not found"}
	}
	return &envelope.SSHKey[0], nil
}

// Create adds a new SSH public key to the account.
func (s SSHKeyService) Create(ctx context.Context, req CreateSSHKeyRequest) (*SSHKey, error) {
	var envelope struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		SSHKey  SSHKey `json:"sshKey"`
	}
	if err := s.client.do(ctx, http.MethodPost, "/webarenaIndigo/v1/vm/sshkey", req, &envelope, true); err != nil {
		return nil, err
	}
	return &envelope.SSHKey, nil
}

// Update modifies the name, public key, or status of an existing SSH key.
func (s SSHKeyService) Update(ctx context.Context, id int, req UpdateSSHKeyRequest) error {
	path := fmt.Sprintf("/webarenaIndigo/v1/vm/sshkey/%d", id)
	return s.client.do(ctx, http.MethodPut, path, req, nil, true)
}

// Delete removes an SSH key from the account.
func (s SSHKeyService) Delete(ctx context.Context, id int) error {
	path := fmt.Sprintf("/webarenaIndigo/v1/vm/sshkey/%d", id)
	return s.client.do(ctx, http.MethodDelete, path, nil, nil, true)
}
