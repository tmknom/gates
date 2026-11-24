package authz_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repositorypolicy"
)

//trace:checks AC-ISSUANCE-010
func TestGrant(t *testing.T) {
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	requested := map[string]string{"contents": "write", "issues": "write"}
	req := mustTokenRequest(t, []string{"Example-Org/App", "example-org/lib"}, requested)
	inst := mustNewInstallation(t, allowedOwnerID, false)

	grant, err := authz.Grant(caller, req, owner, mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced, inst)
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

	grant, err := authz.Grant(mustNewCaller(t, allowedOwnerID), req, mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName), mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced, mustNewInstallation(t, allowedOwnerID, false))
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
			_, err := authz.Grant(caller, req, owner, issuablePermissions, noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced, tt.installation(t))
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

// Grant も保護対象の条件を判定し直すこと（INV-011）を、有効な Installation で確かめる。
//
//trace:checks AC-AUTHZ-011
func TestGrant_DeniedByProtectedRepositories(t *testing.T) {
	req := mustTokenRequest(t, []string{"Example-Org/vault"}, contentsRead)
	_, err := authz.Grant(mustNewCaller(t, allowedOwnerID), req, mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName), mustNewIssuablePermissions(t, issuablePermissionNotations...), mustParseProtectedRepositories(t, protectedRepositoriesDocument), authz.RepositoryPolicyNotEnforced, mustNewInstallation(t, allowedOwnerID, false))
	if !errors.Is(err, authz.ErrAuthorizationDenied) {
		t.Fatalf("Grant() error = %v, want %v", err, authz.ErrAuthorizationDenied)
	}
	if !strings.Contains(err.Error(), "protected repositories") {
		t.Errorf("Grant() error = %q, want it to contain %q", err, "protected repositories")
	}
}

func TestGrant_ErrorDoesNotContainValues(t *testing.T) {
	const installationOwnerID = 987654321
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	caller := mustNewCaller(t, allowedOwnerID)
	req := mustTokenRequest(t, []string{"Example-Org/app"}, contentsRead)
	_, err := authz.Grant(caller, req, owner, mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced, mustNewInstallation(t, installationOwnerID, false))
	if err == nil {
		t.Fatal("Grant() returned no error")
	}
	for _, s := range []string{strconv.Itoa(installationID), strconv.Itoa(installationOwnerID), strconv.Itoa(allowedOwnerID)} {
		if strings.Contains(err.Error(), s) {
			t.Errorf("error %q contains value %q", err, s)
		}
	}
}

// 適用するデプロイで、リポジトリポリシーの条件を判定しない発行許可を確定させない（INV-011）。
//
//trace:checks AC-AUTHZ-017
func TestGrant_DeniedByRepositoryPolicyEnforcement(t *testing.T) {
	tests := []struct {
		name        string
		enforcement authz.RepositoryPolicyEnforcement
		wantReason  string
	}{
		{
			name:        "request under repository policy enforcement is denied",
			enforcement: authz.RepositoryPolicyEnforced,
			wantReason:  "authorization denied: token grant requires repository policies under repository policy enforcement",
		},
		{
			name:        "zero repository policy enforcement is denied as not initialized",
			enforcement: authz.RepositoryPolicyEnforcement{},
			wantReason:  "authorization denied: repository policy enforcement is not initialized",
		},
	}
	owner := mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName)
	issuablePermissions := mustNewIssuablePermissions(t, issuablePermissionNotations...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustTokenRequest(t, []string{"Example-Org/app"}, contentsRead)
			grant, err := authz.Grant(mustNewCaller(t, allowedOwnerID), req, owner, issuablePermissions, noProtectedRepositories(t), tt.enforcement, mustNewInstallation(t, allowedOwnerID, false))
			if !errors.Is(err, authz.ErrAuthorizationDenied) {
				t.Fatalf("Grant() error = %v, want %v", err, authz.ErrAuthorizationDenied)
			}
			if err.Error() != tt.wantReason {
				t.Errorf("Grant() error = %q, want %q", err, tt.wantReason)
			}
			if !grant.IsZero() {
				t.Error("Grant() returned a non-zero TokenGrant with an error")
			}
		})
	}
}

// policyAllowedPermissions は、テストのリポジトリポリシーの許可 Caller の権限の上限である。
// 発行上限より高い水準（pull_requests:write）を含み、発行上限が含む組（issues:write）を含まない。
var policyAllowedPermissions = map[string]string{"contents": "write", "pull_requests": "write"}

//trace:checks AC-AUTHZ-014
func TestGrantWithRepositoryPolicies(t *testing.T) {
	requested := map[string]string{"contents": "write"}
	req := mustTokenRequest(t, []string{"Example-Org/App", "example-org/lib"}, requested)
	allowing := repositoryPolicyAllowing(callerRepositoryID, callerRepositoryName, policyAllowedPermissions)

	grant, err := authz.GrantWithRepositoryPolicies(mustNewCaller(t, allowedOwnerID), req, mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName), mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), authz.RepositoryPolicyEnforced, mustNewInstallation(t, allowedOwnerID, false), repeatFetchResult(2, allowing))
	if err != nil {
		t.Fatalf("GrantWithRepositoryPolicies() returned error: %v", err)
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

// Caller の自分のリポジトリも、ほかの対象と同じくリポジトリポリシーで判定し、許されれば発行する。
//
//trace:checks AC-AUTHZ-014
func TestGrantWithRepositoryPolicies_CallerOwnRepository(t *testing.T) {
	req := mustTokenRequest(t, []string{"Example-Org/" + callerRepositoryName}, contentsRead)
	allowing := repositoryPolicyAllowing(callerRepositoryID, callerRepositoryName, policyAllowedPermissions)

	grant, err := authz.GrantWithRepositoryPolicies(mustNewCaller(t, allowedOwnerID), req, mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName), mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), authz.RepositoryPolicyEnforced, mustNewInstallation(t, allowedOwnerID, false), repeatFetchResult(1, allowing))
	if err != nil {
		t.Fatalf("GrantWithRepositoryPolicies() returned error: %v", err)
	}
	assertRepositories(t, grant.Repositories(), []string{"Example-Org/" + callerRepositoryName})
}

// 拒否の理由の区別と文言は repositorypolicy が検証する。ここでは、対象の位置を添えた拒否、取得の結果の件数、条件の順序を確かめる。
//
//trace:checks AC-AUTHZ-014 AC-AUTHZ-015 AC-AUTHZ-016
func TestGrantWithRepositoryPolicies_Denied(t *testing.T) {
	allowing := repositoryPolicyAllowing(callerRepositoryID, callerRepositoryName, policyAllowedPermissions)
	notMatching := repositoryPolicyAllowing(9009, "other-tools", policyAllowedPermissions)
	tests := []struct {
		name         string
		repositories []string
		permissions  map[string]string
		protected    string
		enforcement  authz.RepositoryPolicyEnforcement
		suspended    bool
		fetchResults []repositorypolicy.FetchResult
		want         error
		notWant      error
		wantReason   string
	}{
		{
			name:         "second repository whose policy has no matching allowed caller is denied with its position",
			repositories: []string{"Example-Org/app", "Example-Org/lib"}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{allowing, notMatching},
			wantReason:   "authorization denied: repositories[1]: repository policy has no allowed caller",
		},
		{
			name:         "first and second repositories whose policies deny are denied by the first",
			repositories: []string{"Example-Org/app", "Example-Org/lib"}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{repositorypolicy.NotFound(), repositorypolicy.NotFile()},
			wantReason:   "authorization denied: repositories[0]: repository policy is not found",
		},
		{
			name:         "repository with invalid policy is denied",
			repositories: []string{"Example-Org/app"}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{repositorypolicy.Document("callers: [\n")},
			wantReason:   "authorization denied: repositories[0]: repository policy is invalid",
		},
		{
			name:         "zero fetch result is denied as not initialized",
			repositories: []string{"Example-Org/app"}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{{}},
			wantReason:   "authorization denied: repositories[0]: repository policy fetch result is not initialized",
		},
		{
			name:         "one fewer fetch result than repositories is denied",
			repositories: []string{"Example-Org/app", "Example-Org/lib"}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{allowing},
			wantReason:   "authorization denied: repository policy fetch results count does not match repositories",
		},
		{
			name:         "one more fetch result than repositories is denied",
			repositories: []string{"Example-Org/app"}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{allowing, allowing},
			wantReason:   "authorization denied: repository policy fetch results count does not match repositories",
		},
		{
			name:         "caller own repository whose policy does not allow the caller is denied",
			repositories: []string{"Example-Org/" + callerRepositoryName}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{notMatching},
			wantReason:   "authorization denied: repositories[0]: repository policy has no allowed caller",
		},
		{
			name:         "protected repository allowed by protected repositories but not by its policy is denied by the policy",
			repositories: []string{"Example-Org/app"}, permissions: map[string]string{"contents": "write"},
			protected:    protectedRepositoriesDocument,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{notMatching},
			wantReason:   "authorization denied: repositories[0]: repository policy has no allowed caller",
		},
		{
			name:         "protected repository allowed by its policy but not by protected repositories is denied by protected repositories",
			repositories: []string{"Example-Org/vault"}, permissions: contentsRead,
			protected:    protectedRepositoriesDocument,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{allowing},
			wantReason:   "matches protected repositories[",
		},
		{
			name:         "permission within issuable permissions but outside the permission limit of the policy is denied by the policy",
			repositories: []string{"Example-Org/app"}, permissions: map[string]string{"issues": "write"},
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{allowing},
			wantReason:   "authorization denied: repositories[0]: permissions are not within the permission limit of the repository policy",
		},
		{
			name:         "permission within the permission limit of the policy but outside issuable permissions is denied by issuable permissions",
			repositories: []string{"Example-Org/app"}, permissions: map[string]string{"pull_requests": "write"},
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{allowing},
			wantReason:   "authorization denied: permissions are not within issuable permissions",
		},
		{
			name:         "request failing the owner name and its policy is denied by the owner name",
			repositories: []string{"other-org/app"}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{notMatching},
			wantReason:   "repositories[0] owner name",
		},
		{
			name:         "suspended installation with a denying policy returns ErrInstallationUnavailable",
			repositories: []string{"Example-Org/app"}, permissions: contentsRead,
			suspended:    true,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{notMatching},
			want:         authz.ErrInstallationUnavailable,
			notWant:      authz.ErrAuthorizationDenied,
			wantReason:   "suspended",
		},
		{
			name:         "suspended installation with fewer fetch results returns ErrInstallationUnavailable",
			repositories: []string{"Example-Org/app", "Example-Org/lib"}, permissions: contentsRead,
			suspended:    true,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: []repositorypolicy.FetchResult{allowing},
			want:         authz.ErrInstallationUnavailable,
			notWant:      authz.ErrAuthorizationDenied,
			wantReason:   "suspended",
		},
		{
			name:         "request without repository policy enforcement is denied",
			repositories: []string{"Example-Org/app"}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyNotEnforced,
			fetchResults: []repositorypolicy.FetchResult{allowing},
			wantReason:   "authorization denied: repository policies are not used without repository policy enforcement",
		},
		{
			name:         "zero repository policy enforcement is denied as not initialized",
			repositories: []string{"Example-Org/app"}, permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforcement{},
			fetchResults: []repositorypolicy.FetchResult{allowing},
			wantReason:   "authorization denied: repository policy enforcement is not initialized",
		},
		{
			name:         "11 repositories allowed by their policies are denied by the count limit",
			repositories: repositoriesOf(11), permissions: contentsRead,
			enforcement:  authz.RepositoryPolicyEnforced,
			fetchResults: repeatFetchResult(11, allowing),
			wantReason:   "authorization denied: repositories count exceeds the limit under repository policy enforcement",
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
			want, notWant := tt.want, tt.notWant
			if want == nil {
				want, notWant = authz.ErrAuthorizationDenied, authz.ErrInstallationUnavailable
			}
			req := mustTokenRequest(t, tt.repositories, tt.permissions)
			grant, err := authz.GrantWithRepositoryPolicies(mustNewCaller(t, allowedOwnerID), req, owner, issuablePermissions, protectedRepositories, tt.enforcement, mustNewInstallation(t, allowedOwnerID, tt.suspended), tt.fetchResults)
			if !errors.Is(err, want) {
				t.Fatalf("GrantWithRepositoryPolicies() error = %v, want %v", err, want)
			}
			if errors.Is(err, notWant) {
				t.Errorf("GrantWithRepositoryPolicies() error = %v, must not be %v", err, notWant)
			}
			if !strings.Contains(err.Error(), tt.wantReason) {
				t.Errorf("GrantWithRepositoryPolicies() error = %q, want it to contain %q", err, tt.wantReason)
			}
			if !grant.IsZero() {
				t.Error("GrantWithRepositoryPolicies() returned a non-zero TokenGrant with an error")
			}
		})
	}
}

func TestGrantWithRepositoryPolicies_ErrorDoesNotContainValues(t *testing.T) {
	const (
		secretCallerID         = 7654321
		secretCallerName       = "secret-caller"
		secretAllowedCallerID  = 1234567
		secretAllowedCaller    = "secret-allowed"
		secretTarget           = "Example-Org/secret-target" //nolint:gosec // 資格情報ではなく、漏れを探すための目印である
		secretDocumentContents = "secret-document"
	)
	tests := []struct {
		name        string
		fetchResult repositorypolicy.FetchResult
	}{
		{name: "policy without matching allowed caller does not reveal the policy and the caller", fetchResult: repositoryPolicyAllowing(secretAllowedCallerID, secretAllowedCaller, contentsRead)},
		{name: "invalid policy does not reveal the document", fetchResult: repositorypolicy.Document("callers: " + secretDocumentContents + "\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mustTokenRequest(t, []string{secretTarget}, contentsRead)
			_, err := authz.GrantWithRepositoryPolicies(mustNewCallerOf(t, secretCallerID, secretCallerName), req, mustNewAllowedOwner(t, allowedOwnerID, allowedOwnerName), mustNewIssuablePermissions(t, issuablePermissionNotations...), noProtectedRepositories(t), authz.RepositoryPolicyEnforced, mustNewInstallation(t, allowedOwnerID, false), []repositorypolicy.FetchResult{tt.fetchResult})
			if err == nil {
				t.Fatal("GrantWithRepositoryPolicies() returned no error")
			}
			for _, s := range []string{strconv.Itoa(secretCallerID), secretCallerName, strconv.Itoa(secretAllowedCallerID), secretAllowedCaller, "secret-target", allowedOwnerName, secretDocumentContents} {
				if strings.Contains(err.Error(), s) {
					t.Errorf("error %q contains value %q", err, s)
				}
			}
		})
	}
}
