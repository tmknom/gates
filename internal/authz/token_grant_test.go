package authz_test

import (
	"testing"

	"github.com/tmknom/gates/internal/authz"
)

func TestTokenGrant_IsZero(t *testing.T) {
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	req := mustTokenRequest(t, []string{"Example-Org/app"}, contentsRead)
	grant, err := authz.Grant(caller, req, owner, mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), mustNewInstallation(t, allowedOwnerID, false))
	if err != nil {
		t.Fatalf("Grant() returned error: %v", err)
	}

	if grant.IsZero() {
		t.Error("IsZero() = true for the TokenGrant created by Grant")
	}
	if !(authz.TokenGrant{}).IsZero() {
		t.Error("IsZero() = false for the zero TokenGrant")
	}
}

func TestTokenGrant_RepositoriesCannotBeModifiedByCaller(t *testing.T) {
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	req := mustTokenRequest(t, []string{"Example-Org/app", "Example-Org/lib"}, contentsRead)
	grant, err := authz.Grant(caller, req, owner, mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), mustNewInstallation(t, allowedOwnerID, false))
	if err != nil {
		t.Fatalf("Grant() returned error: %v", err)
	}
	got := grant.Repositories()
	got[0] = got[1]
	assertRepositories(t, grant.Repositories(), []string{"Example-Org/app", "Example-Org/lib"})
}
