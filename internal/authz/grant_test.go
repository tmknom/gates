package authz_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/permission"
)

//trace:checks AC-ISSUANCE-010
func TestGrant(t *testing.T) {
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	requested := map[string]string{"contents": "write", "issues": "write"}
	req := mustTokenRequest(t, []string{"Example-Org/App", "example-org/lib"}, requested)
	inst := mustNewInstallation(t, allowedOwnerID, false)

	grant, err := authz.Grant(caller, req, owner, mustNewIssuablePermissions(t, issuablePermissionNotations...), inst)
	if err != nil {
		t.Fatalf("Grant() returned error: %v", err)
	}
	if grant.InstallationID().Int64() != installationID {
		t.Errorf("InstallationID() = %d, want %d", grant.InstallationID().Int64(), installationID)
	}
	assertRepositories(t, grant.Repositories(), []string{"Example-Org/App", "example-org/lib"})
	want, err := permission.NewPermissions(requested)
	if err != nil {
		t.Fatalf("NewPermissions returned error: %v", err)
	}
	if !grant.Permissions().Equal(want) {
		t.Error("Permissions() is not equal to the requested permissions")
	}
}

// 発行上限が高い水準だけを含む権限名の低い水準の要求で、発行許可の Permissions が上限の水準に上がらないことを確かめる。
//
//trace:checks AC-AUTHZ-009
func TestGrant_KeepsRequestedLevelBelowIssuableLevel(t *testing.T) {
	requested := map[string]string{"issues": "read"}
	req := mustTokenRequest(t, []string{"Example-Org/app"}, requested)

	grant, err := authz.Grant(mustNewCaller(t, allowedOwnerID), req, mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName), mustNewIssuablePermissions(t, issuablePermissionNotations...), mustNewInstallation(t, allowedOwnerID, false))
	if err != nil {
		t.Fatalf("Grant() returned error: %v", err)
	}
	want, err := permission.NewPermissions(requested)
	if err != nil {
		t.Fatalf("NewPermissions returned error: %v", err)
	}
	if !grant.Permissions().Equal(want) {
		t.Error("Permissions() is not equal to the requested permissions")
	}
}

//trace:checks AC-AUTHZ-004 AC-AUTHZ-009
func TestGrant_Denied(t *testing.T) {
	tests := []struct {
		name          string
		callerOwnerID int64
		permissions   map[string]string
		installation  func(t *testing.T) authz.Installation
		want          error
		notWant       error
		// 監査記録で満たさなかった条件を読み取れることを確かめる
		wantReason string
	}{
		{
			name:          "installation owner id different from allowed owner id returns ErrInstallationUnavailable",
			callerOwnerID: allowedOwnerID,
			permissions:   contentsRead,
			installation:  func(t *testing.T) authz.Installation { return mustNewInstallation(t, 2002, false) },
			want:          authz.ErrInstallationUnavailable,
			notWant:       authz.ErrAuthorizationDenied,
			wantReason:    "installation owner id",
		},
		{
			name:          "suspended installation returns ErrInstallationUnavailable",
			callerOwnerID: allowedOwnerID,
			permissions:   contentsRead,
			installation:  func(t *testing.T) authz.Installation { return mustNewInstallation(t, allowedOwnerID, true) },
			want:          authz.ErrInstallationUnavailable,
			notWant:       authz.ErrAuthorizationDenied,
			wantReason:    "suspended",
		},
		{
			name:          "permission outside issuable permissions with usable installation returns ErrAuthorizationDenied",
			callerOwnerID: allowedOwnerID,
			permissions:   map[string]string{"deployments": "read"},
			installation:  func(t *testing.T) authz.Installation { return mustNewInstallation(t, allowedOwnerID, false) },
			want:          authz.ErrAuthorizationDenied,
			notWant:       authz.ErrInstallationUnavailable,
			wantReason:    "issuable permissions",
		},
		{
			name:          "request failing both authorization and installation validity returns ErrAuthorizationDenied",
			callerOwnerID: 2002,
			permissions:   contentsRead,
			installation:  func(t *testing.T) authz.Installation { return mustNewInstallation(t, 2002, true) },
			want:          authz.ErrAuthorizationDenied,
			notWant:       authz.ErrInstallationUnavailable,
		},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := mustNewCaller(t, tt.callerOwnerID)
			req := mustTokenRequest(t, []string{"Example-Org/app"}, tt.permissions)
			_, err := authz.Grant(caller, req, owner, issuablePermissions, tt.installation(t))
			if !errors.Is(err, tt.want) {
				t.Fatalf("Grant() error = %v, want %v", err, tt.want)
			}
			if errors.Is(err, tt.notWant) {
				t.Errorf("Grant() error = %v, must not be %v", err, tt.notWant)
			}
			if !strings.Contains(err.Error(), tt.wantReason) {
				t.Errorf("Grant() error = %q, want it to contain %q", err, tt.wantReason)
			}
		})
	}
}

func TestGrant_ErrorDoesNotContainValues(t *testing.T) {
	const installationOwnerID = 987654321
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	req := mustTokenRequest(t, []string{"Example-Org/app"}, contentsRead)
	_, err := authz.Grant(caller, req, owner, mustNewIssuablePermissions(t, issuablePermissionNotations...), mustNewInstallation(t, installationOwnerID, false))
	if err == nil {
		t.Fatal("Grant() returned no error")
	}
	for _, s := range []string{strconv.Itoa(installationID), strconv.Itoa(installationOwnerID), strconv.Itoa(allowedOwnerID)} {
		if strings.Contains(err.Error(), s) {
			t.Errorf("error %q contains value %q", err, s)
		}
	}
}
