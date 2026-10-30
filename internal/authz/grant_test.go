package authz_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

const installationID = 5005

var suspendedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func TestGrant(t *testing.T) {
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	req := mustTokenRequest(t, []string{"Example-Org/App", "example-org/lib"}, contentsRead)
	inst := mustNewInstallation(t, allowedOwnerID, time.Time{})

	grant, err := authz.Grant(caller, req, owner, inst)
	if err != nil {
		t.Fatalf("Grant() returned error: %v", err)
	}
	if grant.InstallationID() != installationID {
		t.Errorf("InstallationID() = %d, want %d", grant.InstallationID(), installationID)
	}
	assertRepositories(t, grant.Repositories(), []string{"Example-Org/App", "example-org/lib"})
	if !grant.Permissions().Equal(permission.AllowedPermissions()) {
		t.Error("Permissions() is not equal to allowed permissions")
	}
}

func TestTokenGrant_RepositoriesCannotBeModifiedByCaller(t *testing.T) {
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	req := mustTokenRequest(t, []string{"Example-Org/app", "Example-Org/lib"}, contentsRead)
	grant, err := authz.Grant(caller, req, owner, mustNewInstallation(t, allowedOwnerID, time.Time{}))
	if err != nil {
		t.Fatalf("Grant() returned error: %v", err)
	}
	got := grant.Repositories()
	got[0] = got[1]
	assertRepositories(t, grant.Repositories(), []string{"Example-Org/app", "Example-Org/lib"})
}

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
			name:          "installation account id different from allowed owner id returns ErrInstallationUnavailable",
			callerOwnerID: allowedOwnerID,
			permissions:   contentsRead,
			installation:  func(t *testing.T) authz.Installation { return mustNewInstallation(t, 2002, time.Time{}) },
			want:          authz.ErrInstallationUnavailable,
			notWant:       authz.ErrAuthorizationDenied,
			wantReason:    "account id",
		},
		{
			name:          "suspended installation returns ErrInstallationUnavailable",
			callerOwnerID: allowedOwnerID,
			permissions:   contentsRead,
			installation:  func(t *testing.T) authz.Installation { return mustNewInstallation(t, allowedOwnerID, suspendedAt) },
			want:          authz.ErrInstallationUnavailable,
			notWant:       authz.ErrAuthorizationDenied,
			wantReason:    "suspended",
		},
		{
			name:          "request failing authorization with usable installation returns ErrAuthorizationDenied",
			callerOwnerID: allowedOwnerID,
			permissions:   map[string]string{"contents": "write"},
			installation:  func(t *testing.T) authz.Installation { return mustNewInstallation(t, allowedOwnerID, time.Time{}) },
			want:          authz.ErrAuthorizationDenied,
			notWant:       authz.ErrInstallationUnavailable,
		},
		{
			name:          "request failing both authorization and installation validity returns ErrAuthorizationDenied",
			callerOwnerID: 2002,
			permissions:   contentsRead,
			installation:  func(t *testing.T) authz.Installation { return mustNewInstallation(t, 2002, suspendedAt) },
			want:          authz.ErrAuthorizationDenied,
			notWant:       authz.ErrInstallationUnavailable,
		},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := mustNewCaller(t, tt.callerOwnerID)
			req := mustTokenRequest(t, []string{"Example-Org/app"}, tt.permissions)
			_, err := authz.Grant(caller, req, owner, tt.installation(t))
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
	const accountID = 987654321
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	req := mustTokenRequest(t, []string{"Example-Org/app"}, contentsRead)
	_, err := authz.Grant(caller, req, owner, mustNewInstallation(t, accountID, time.Time{}))
	if err == nil {
		t.Fatal("Grant() returned no error")
	}
	for _, s := range []string{strconv.Itoa(installationID), strconv.Itoa(accountID), strconv.Itoa(allowedOwnerID)} {
		if strings.Contains(err.Error(), s) {
			t.Errorf("error %q contains value %q", err, s)
		}
	}
}

func mustNewInstallation(t *testing.T, accountID int64, suspendedAt time.Time) authz.Installation {
	t.Helper()
	inst, err := authz.NewInstallation(installationID, accountID, suspendedAt)
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	return inst
}

func assertRepositories(t *testing.T, got []repository.FullName, want []string) {
	t.Helper()
	// FullName.Equal は大文字小文字を区別しないため、要求どおりの表記が保たれたかは String() の列で比べる
	names := make([]string, len(got))
	for i, f := range got {
		names[i] = f.String()
	}
	if !slices.Equal(names, want) {
		t.Errorf("Repositories() = %v, want %v", names, want)
	}
}
