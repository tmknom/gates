package authz_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/protected"
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
		{name: "lower level of permission name with only higher issuable level is authorized", repositories: []string{"Example-Org/app"}, permissions: map[string]string{"issues": "read"}},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			if err := authz.Authorize(caller, req, owner, issuablePermissions, noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced); err != nil {
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
		{name: "allowed permission outside issuable permissions is denied by issuable permissions", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"deployments": "read"}, wantReason: "issuable permissions"},
		{name: "higher level of permission name with only lower issuable level is denied by issuable permissions", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"contents": "read", "pull_requests": "write"}, wantReason: "issuable permissions"},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := mustNewCaller(t, tt.callerOwnerID)
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			err := authz.Authorize(caller, req, owner, issuablePermissions, noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced)
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
	err := authz.Authorize(caller.Caller{}, req, authz.AllowedOwner{}, mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced)
	if !errors.Is(err, authz.ErrAuthorizationDenied) {
		t.Errorf("Authorize() error = %v, want %v", err, authz.ErrAuthorizationDenied)
	}
}

//trace:checks AC-AUTHZ-011
func TestAuthorize_ProtectedRepositories(t *testing.T) {
	tests := []struct {
		name         string
		repositories []string
		permissions  map[string]string
	}{
		{name: "allowed caller requesting the level of its permission limit is authorized", repositories: []string{"Example-Org/app"}, permissions: map[string]string{"contents": "write"}},
		{name: "request without protected repositories is authorized", repositories: []string{"Example-Org/lib"}, permissions: map[string]string{"issues": "write"}},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	protectedRepositories := mustParseProtectedRepositories(t, protectedRepositoriesDocument)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			if err := authz.Authorize(caller, req, owner, issuablePermissions, protectedRepositories, authz.RepositoryPolicyNotEnforced); err != nil {
				t.Errorf("Authorize() returned error: %v", err)
			}
		})
	}
}

// 拒否の理由の区別と文言は protected が検証する。ここでは、保護対象の条件で拒否することと、条件の順序を確かめる
//
//trace:checks AC-AUTHZ-011
func TestAuthorize_ProtectedRepositoriesDenied(t *testing.T) {
	// protectedRepositoriesReason は、保護対象の条件による拒否の文言のうち、ほかの条件の文言と見分けられる部分である
	const protectedRepositoriesReason = "matches protected repositories["
	tests := []struct {
		name               string
		callerRepositoryID int64
		callerName         string
		repositories       []string
		permissions        map[string]string
		wantReason         string
	}{
		{
			name:               "caller matching the allowed caller id but not the name is denied by the protected repository condition",
			callerRepositoryID: callerRepositoryID, callerName: "renamed-tools",
			repositories: []string{"Example-Org/app"}, permissions: contentsRead,
			wantReason: protectedRepositoriesReason,
		},
		{
			name:               "permission within issuable permissions but outside the permission limit is denied by the protected repository condition",
			callerRepositoryID: callerRepositoryID, callerName: callerRepositoryName,
			repositories: []string{"Example-Org/app"}, permissions: map[string]string{"issues": "write"},
			wantReason: protectedRepositoriesReason,
		},
		{
			name:               "permission within the permission limit but outside issuable permissions is denied by issuable permissions",
			callerRepositoryID: callerRepositoryID, callerName: callerRepositoryName,
			repositories: []string{"Example-Org/app"}, permissions: map[string]string{"pull_requests": "write"},
			wantReason: "issuable permissions",
		},
		{
			name:               "permission outside issuable permissions and the permission limit is denied by issuable permissions",
			callerRepositoryID: callerRepositoryID, callerName: callerRepositoryName,
			repositories: []string{"Example-Org/app"}, permissions: map[string]string{"deployments": "read"},
			wantReason: "issuable permissions",
		},
		{
			name:               "request failing the owner name and the protected repository condition is denied by the owner name",
			callerRepositoryID: 9009, callerName: "other",
			repositories: []string{"other-org/app"}, permissions: contentsRead,
			wantReason: "repositories[0] owner name",
		},
		{
			name:               "protected repository after an unprotected one is denied with its position",
			callerRepositoryID: callerRepositoryID, callerName: callerRepositoryName,
			repositories: []string{"Example-Org/lib", "Example-Org/vault"}, permissions: contentsRead,
			wantReason: "repositories[1] " + protectedRepositoriesReason,
		},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	protectedRepositories := mustParseProtectedRepositories(t, protectedRepositoriesDocument)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := mustNewCallerOf(t, tt.callerRepositoryID, tt.callerName)
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			err := authz.Authorize(caller, req, owner, issuablePermissions, protectedRepositories, authz.RepositoryPolicyNotEnforced)
			if !errors.Is(err, authz.ErrAuthorizationDenied) {
				t.Fatalf("Authorize() error = %v, want %v", err, authz.ErrAuthorizationDenied)
			}
			if !strings.Contains(err.Error(), tt.wantReason) {
				t.Errorf("Authorize() error = %q, want it to contain %q", err, tt.wantReason)
			}
		})
	}
}

// ゼロ値の一覧を保護対象なしと読むと、初期化の漏れで保護が外れるため、保護対象を含まない要求も拒否されることを確かめる。
func TestAuthorize_ZeroProtectedRepositoriesAreDenied(t *testing.T) {
	req := mustTokenRequest(t, []string{"Example-Org/lib"}, contentsRead)
	err := authz.Authorize(mustNewCaller(t, allowedOwnerID), req, mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName), mustNewIssuablePermissions(t, issuablePermissionNotations...), protected.Repositories{}, authz.RepositoryPolicyNotEnforced)
	if !errors.Is(err, authz.ErrAuthorizationDenied) {
		t.Errorf("Authorize() error = %v, want %v", err, authz.ErrAuthorizationDenied)
	}
}

//trace:checks AC-AUTHZ-017
func TestAuthorize_RepositoryCountLimit(t *testing.T) {
	tests := []struct {
		name         string
		enforcement  authz.RepositoryPolicyEnforcement
		repositories []string
	}{
		{name: "11 repositories without repository policy enforcement are authorized", enforcement: authz.RepositoryPolicyNotEnforced, repositories: repositoriesOf(11)},
		{name: "10 repositories under repository policy enforcement are authorized", enforcement: authz.RepositoryPolicyEnforced, repositories: repositoriesOf(10)},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustTokenRequest(t, tt.repositories, contentsRead)
			if err := authz.Authorize(caller, req, owner, issuablePermissions, noProtectedRepositories(t), tt.enforcement); err != nil {
				t.Errorf("Authorize() returned error: %v", err)
			}
		})
	}
}

//trace:checks AC-AUTHZ-017
func TestAuthorize_RepositoryCountLimitDenied(t *testing.T) {
	tests := []struct {
		name         string
		enforcement  authz.RepositoryPolicyEnforcement
		repositories []string
		wantReason   string
	}{
		{
			name:         "11 repositories under repository policy enforcement are denied by the count limit",
			enforcement:  authz.RepositoryPolicyEnforced,
			repositories: repositoriesOf(11),
			wantReason:   "authorization denied: repositories count exceeds the limit under repository policy enforcement",
		},
		{
			name:         "11 repositories with a different owner name under repository policy enforcement are denied by the owner name",
			enforcement:  authz.RepositoryPolicyEnforced,
			repositories: append(repositoriesOf(10), "other-org/app"),
			wantReason:   "repositories[10] owner name",
		},
		{
			name:         "zero repository policy enforcement is denied as not initialized",
			enforcement:  authz.RepositoryPolicyEnforcement{},
			repositories: repositoriesOf(1),
			wantReason:   "authorization denied: repository policy enforcement is not initialized",
		},
		{
			name:         "zero repository policy enforcement with a different owner name is denied by the owner name",
			enforcement:  authz.RepositoryPolicyEnforcement{},
			repositories: []string{"other-org/app"},
			wantReason:   "repositories[0] owner name",
		},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustTokenRequest(t, tt.repositories, contentsRead)
			err := authz.Authorize(caller, req, owner, issuablePermissions, noProtectedRepositories(t), tt.enforcement)
			if !errors.Is(err, authz.ErrAuthorizationDenied) {
				t.Fatalf("Authorize() error = %v, want %v", err, authz.ErrAuthorizationDenied)
			}
			if !strings.Contains(err.Error(), tt.wantReason) {
				t.Errorf("Authorize() error = %q, want it to contain %q", err, tt.wantReason)
			}
		})
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
		{name: "permission outside issuable permissions does not reveal permission names", callerOwnerID: allowedOwnerID, repositories: []string{"Example-Org/app"}, permissions: map[string]string{"deployments": "read"}},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := mustNewCaller(t, tt.callerOwnerID)
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			err := authz.Authorize(caller, req, owner, issuablePermissions, noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced)
			if err == nil {
				t.Fatal("Authorize() returned no error")
			}
			for _, s := range []string{strconv.FormatInt(allowedOwnerID, 10), strconv.FormatInt(tt.callerOwnerID, 10), allowedOwnerName, "secret-requested", "secret_requested", "deployments"} {
				if strings.Contains(err.Error(), s) {
					t.Errorf("error %q contains value %q", err, s)
				}
			}
		})
	}
}
