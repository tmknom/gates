package authz_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
)

//trace:checks AC-ISSUANCE-002 AC-AUTHZ-009
func TestAuthorize(t *testing.T) {
	tests := []struct {
		name         string
		repositories []string
		permissions  map[string]string
	}{
		{name: "single repository of allowed owner is authorized", repositories: []string{"Example-Org/app"}, permissions: contentsRead},
		{name: "multiple repositories of allowed owner are authorized", repositories: []string{"Example-Org/app", "Example-Org/lib"}, permissions: contentsRead},
		{name: "owner name differing only in case is authorized", repositories: []string{"example-org/app"}, permissions: contentsRead},
		{name: "multiple permissions with write levels in issuable permissions are authorized", repositories: []string{"Example-Org/app"}, permissions: map[string]string{"contents": "write", "issues": "write"}},
		{name: "requested metadata read in issuable permissions is authorized", repositories: []string{"Example-Org/app"}, permissions: map[string]string{"contents": "read", "metadata": "read"}},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			if err := authz.Authorize(caller, req, owner, issuablePermissions); err != nil {
				t.Errorf("Authorize() returned error: %v", err)
			}
		})
	}
}

//trace:checks AC-AUTHN-001 AC-AUTHZ-001 AC-AUTHZ-003 AC-AUTHZ-004 AC-AUTHZ-005 AC-AUTHZ-006 AC-AUTHZ-009
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
		{name: "organization permission is denied by allowed permissions", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"members": "read"}, wantReason: "allowed permissions"},
		{name: "allowed permission outside issuable permissions is denied by issuable permissions", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"pull_requests": "read"}, wantReason: "issuable permissions"},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := mustNewCaller(t, tt.callerOwnerID)
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			err := authz.Authorize(caller, req, owner, issuablePermissions)
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
	err := authz.Authorize(caller.Caller{}, req, authz.AllowedOwner{}, mustNewIssuablePermissions(t, issuablePermissionNotations...))
	if !errors.Is(err, authz.ErrAuthorizationDenied) {
		t.Errorf("Authorize() error = %v, want %v", err, authz.ErrAuthorizationDenied)
	}
}

func TestAuthorize_ErrorDoesNotContainValues(t *testing.T) {
	tests := []struct {
		name          string
		callerOwnerID int64
		repositories  []string
		permissions   map[string]string
	}{
		{name: "owner id mismatch does not reveal ids", callerOwnerID: 123456789, repositories: []string{"Example-Org/app"}, permissions: contentsRead},
		{name: "owner name mismatch does not reveal names", callerOwnerID: allowedOwnerID, repositories: []string{"secret-requested/app"}, permissions: contentsRead},
		{name: "permission outside allowed permissions does not reveal permission names", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"secret_requested": "read"}},
		{name: "permission outside issuable permissions does not reveal permission names", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"pull_requests": "read"}},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := mustNewCaller(t, tt.callerOwnerID)
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			err := authz.Authorize(caller, req, owner, issuablePermissions)
			if err == nil {
				t.Fatal("Authorize() returned no error")
			}
			for _, s := range []string{strconv.FormatInt(allowedOwnerID, 10), strconv.FormatInt(tt.callerOwnerID, 10), allowedOwnerName, "secret-requested", "secret_requested", "pull_requests"} {
				if strings.Contains(err.Error(), s) {
					t.Errorf("error %q contains value %q", err, s)
				}
			}
		})
	}
}
