package authz_test

import (
	"encoding/json/v2"
	"slices"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/request"
)

const (
	allowedOwnerID   = 1001
	allowedOwnerName = "Example-Org"
)

const installationID = 5005

var contentsRead = map[string]string{"contents": "read"}

// issuablePermissionNotations は、テストの発行上限である。同じ権限名の高い水準だけを含む組（`issues:write`）と、付随権限の組を含む。
var issuablePermissionNotations = []string{"contents:read", "contents:write", "issues:write", "metadata:read"}

func mustNewAllowedOwner(t *testing.T, id int64, name string) authz.AllowedOwner {
	t.Helper()
	o, err := authz.NewAllowedOwner(id, name, authz.OwnerTypeOrganization)
	if err != nil {
		t.Fatalf("NewAllowedOwner returned error: %v", err)
	}
	return o
}

func mustNewIssuablePermissions(t *testing.T, notations ...string) permission.IssuablePermissions {
	t.Helper()
	issuablePermissions, err := permission.NewIssuablePermissions(notations)
	if err != nil {
		t.Fatalf("NewIssuablePermissions(%v) returned error: %v", notations, err)
	}
	return issuablePermissions
}

func mustNewCaller(t *testing.T, ownerID int64) caller.Caller {
	t.Helper()
	c, err := caller.NewCaller(ownerID, 3003, 4004, 1, caller.WorkflowTriggerAttributes{})
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

func mustNewInstallation(t *testing.T, installationOwnerID int64, suspended bool) authz.Installation {
	t.Helper()
	inst, err := authz.NewInstallation(installationID, installationOwnerID, suspended)
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
