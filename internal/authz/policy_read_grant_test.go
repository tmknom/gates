package authz_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/permission"
)

func TestGrantPolicyRead(t *testing.T) {
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	req := mustTokenRequest(t, []string{"Example-Org/App", "example-org/lib"}, map[string]string{"contents": "write", "issues": "write"})

	grant, err := authz.GrantPolicyRead(mustNewCaller(t, allowedOwnerID), req, owner, mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), mustNewInstallation(t, allowedOwnerID, false))
	if err != nil {
		t.Fatalf("GrantPolicyRead() returned error: %v", err)
	}
	if grant.InstallationID().Int64() != installationID {
		t.Errorf("InstallationID() = %d, want %d", grant.InstallationID().Int64(), installationID)
	}
	assertRepositories(t, grant.Repositories(), []string{"Example-Org/App", "example-org/lib"})
	// 要求の Permissions によらず、ポリシー読み取り許可の Permissions は single_file:read だけである
	if !grant.Permissions().Equal(permission.PolicyRead()) {
		t.Error("Permissions() is not single_file:read only")
	}
}

func TestGrantPolicyRead_Denied(t *testing.T) {
	tests := []struct {
		name          string
		callerOwnerID int64
		repository    string
		permissions   map[string]string
		protected     string
		inst          func(t *testing.T) authz.Installation
		want          error
		wantReason    string
	}{
		{
			name:          "installation owner id different from allowed owner id returns ErrInstallationUnavailable",
			callerOwnerID: allowedOwnerID,
			repository:    "Example-Org/app",
			permissions:   contentsRead,
			inst:          func(t *testing.T) authz.Installation { return mustNewInstallation(t, 2002, false) },
			want:          authz.ErrInstallationUnavailable,
			wantReason:    "installation owner id",
		},
		{
			name:          "suspended installation returns ErrInstallationUnavailable",
			callerOwnerID: allowedOwnerID,
			repository:    "Example-Org/app",
			permissions:   contentsRead,
			inst:          func(t *testing.T) authz.Installation { return mustNewInstallation(t, allowedOwnerID, true) },
			want:          authz.ErrInstallationUnavailable,
			wantReason:    "suspended",
		},
		{
			name:          "permission outside issuable permissions returns ErrAuthorizationDenied",
			callerOwnerID: allowedOwnerID,
			repository:    "Example-Org/app",
			permissions:   map[string]string{"deployments": "read"},
			inst:          func(t *testing.T) authz.Installation { return mustNewInstallation(t, allowedOwnerID, false) },
			want:          authz.ErrAuthorizationDenied,
			wantReason:    "issuable permissions",
		},
		{
			name:          "protected repository without allowed caller returns ErrAuthorizationDenied",
			callerOwnerID: allowedOwnerID,
			repository:    "Example-Org/vault",
			permissions:   contentsRead,
			protected:     protectedRepositoriesDocument,
			inst:          func(t *testing.T) authz.Installation { return mustNewInstallation(t, allowedOwnerID, false) },
			want:          authz.ErrAuthorizationDenied,
			wantReason:    "protected repositories",
		},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protectedRepositories := noProtectedRepositories(t)
			if tt.protected != "" {
				protectedRepositories = mustParseProtectedRepositories(t, tt.protected)
			}
			req := mustTokenRequest(t, []string{tt.repository}, tt.permissions)
			grant, err := authz.GrantPolicyRead(mustNewCaller(t, tt.callerOwnerID), req, owner, issuablePermissions, protectedRepositories, tt.inst(t))
			if !errors.Is(err, tt.want) {
				t.Fatalf("GrantPolicyRead() error = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.wantReason) {
				t.Errorf("GrantPolicyRead() error = %q, want it to contain %q", err, tt.wantReason)
			}
			if !grant.IsZero() {
				t.Error("GrantPolicyRead() returned a non-zero PolicyReadGrant with an error")
			}
		})
	}
}

func TestPolicyReadGrant_IsZero(t *testing.T) {
	grant, err := authz.GrantPolicyRead(mustNewCaller(t, allowedOwnerID), mustTokenRequest(t, []string{"Example-Org/app"}, contentsRead), mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName), mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), mustNewInstallation(t, allowedOwnerID, false))
	if err != nil {
		t.Fatalf("GrantPolicyRead() returned error: %v", err)
	}
	if grant.IsZero() {
		t.Error("IsZero() = true for the PolicyReadGrant created by GrantPolicyRead")
	}
	if !(authz.PolicyReadGrant{}).IsZero() {
		t.Error("IsZero() = false for the zero PolicyReadGrant")
	}
}

func TestPolicyReadGrant_RepositoriesCannotBeModifiedByCaller(t *testing.T) {
	grant, err := authz.GrantPolicyRead(mustNewCaller(t, allowedOwnerID), mustTokenRequest(t, []string{"Example-Org/app", "Example-Org/lib"}, contentsRead), mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName), mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), mustNewInstallation(t, allowedOwnerID, false))
	if err != nil {
		t.Fatalf("GrantPolicyRead() returned error: %v", err)
	}
	got := grant.Repositories()
	got[0] = got[1]
	assertRepositories(t, grant.Repositories(), []string{"Example-Org/app", "Example-Org/lib"})
}
