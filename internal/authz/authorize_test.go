package authz_test

import (
	"encoding/json/v2"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/request"
)

const (
	allowedOwnerID   = 1001
	allowedOwnerName = "Example-Org"
)

var contentsRead = map[string]string{"contents": "read"}

func TestAuthorize(t *testing.T) {
	tests := []struct {
		name         string
		repositories []string
	}{
		{name: "single repository of allowed owner is authorized", repositories: []string{"Example-Org/app"}},
		{name: "multiple repositories of allowed owner are authorized", repositories: []string{"Example-Org/app", "Example-Org/lib"}},
		{name: "owner name differing only in case is authorized", repositories: []string{"example-org/app"}},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustTokenRequest(t, tt.repositories, contentsRead)
			if err := authz.Authorize(caller, req, owner); err != nil {
				t.Errorf("Authorize() returned error: %v", err)
			}
		})
	}
}

func TestAuthorize_Denied(t *testing.T) {
	tests := []struct {
		name          string
		callerOwnerID int64
		repositories  []string
		permissions   map[string]string
		// 監査記録で満たさなかった条件と要求内の位置を読み取れることを確かめる
		wantReason string
	}{
		{name: "caller owner id different from allowed owner id with matching owner name is denied", callerOwnerID: 2002, repositories: []string{"Example-Org/app"}, permissions: contentsRead, wantReason: "owner id"},
		{name: "one of multiple repositories with different owner name is denied", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app", "other-org/app", "Example-Org/lib"}, permissions: contentsRead, wantReason: "repositories[1] owner name"},
		{name: "contents write is denied", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"contents": "write"}, wantReason: "permissions"},
		{name: "issues read is denied", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"issues": "read"}, wantReason: "permissions"},
		{name: "contents read with metadata read is denied", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"contents": "read", "metadata": "read"}, wantReason: "permissions"},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := mustNewCaller(t, tt.callerOwnerID)
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			err := authz.Authorize(caller, req, owner)
			if !errors.Is(err, authz.ErrAuthorizationDenied) {
				t.Fatalf("Authorize() error = %v, want %v", err, authz.ErrAuthorizationDenied)
			}
			if !strings.Contains(err.Error(), tt.wantReason) {
				t.Errorf("Authorize() error = %q, want it to contain %q", err, tt.wantReason)
			}
		})
	}
}

// ゼロ値どうしは Owner ID が一致してしまうため、形式検証を通った要求がほかの条件で拒否されることを確かめる。
func TestAuthorize_ZeroValueOwnerAndCallerAreDenied(t *testing.T) {
	req := mustTokenRequest(t, []string{"Example-Org/app"}, contentsRead)
	err := authz.Authorize(authz.Caller{}, req, authz.AllowedOwner{})
	if !errors.Is(err, authz.ErrAuthorizationDenied) {
		t.Errorf("Authorize() error = %v, want %v", err, authz.ErrAuthorizationDenied)
	}
}

func TestAuthorize_ErrorDoesNotContainValues(t *testing.T) {
	tests := []struct {
		name          string
		callerOwnerID int64
		repositories  []string
	}{
		{name: "owner id mismatch does not reveal ids", callerOwnerID: 123456789, repositories: []string{"Example-Org/app"}},
		{name: "owner name mismatch does not reveal names", callerOwnerID: allowedOwnerID, repositories: []string{"secret-requested/app"}},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := mustNewCaller(t, tt.callerOwnerID)
			req := mustTokenRequest(t, tt.repositories, contentsRead)
			err := authz.Authorize(caller, req, owner)
			if err == nil {
				t.Fatal("Authorize() returned no error")
			}
			for _, s := range []string{strconv.FormatInt(allowedOwnerID, 10), strconv.FormatInt(tt.callerOwnerID, 10), allowedOwnerName, "secret-requested"} {
				if strings.Contains(err.Error(), s) {
					t.Errorf("error %q contains value %q", err, s)
				}
			}
		})
	}
}

func mustNewAllowedOwner(t *testing.T, id int64, name string) authz.AllowedOwner {
	t.Helper()
	o, err := authz.NewAllowedOwner(id, name, authz.OwnerTypeOrganization)
	if err != nil {
		t.Fatalf("NewAllowedOwner returned error: %v", err)
	}
	return o
}

func mustNewCaller(t *testing.T, ownerID int64) authz.Caller {
	t.Helper()
	c, err := authz.NewCaller(ownerID, 3003, 4004, 1, authz.WorkflowTriggerAttributes{})
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	return c
}

// mustTokenRequest は、TokenRequest を Validate でしか作れないため、Payload を組み立てて形式検証を通す。
func mustTokenRequest(t *testing.T, repositories []string, permissions map[string]string) request.TokenRequest {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"version":      1,
		"id_token":     "header.payload.signature",
		"repositories": repositories,
		"permissions":  permissions,
	})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	_, req, err := request.Validate(b)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return req
}
