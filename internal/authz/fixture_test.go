package authz_test

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/repositorypolicy"
	"github.com/tmknom/gates/internal/request"
)

const (
	allowedOwnerID   = 1001
	allowedOwnerName = "Example-Org"
)

const installationID = 5005

// callerRepositoryID と callerRepositoryName は、mustNewCaller が作る Caller のリポジトリで、protectedRepositoriesDocument の許可 Caller に当たる。
const (
	callerRepositoryID   = 3003
	callerRepositoryName = "deploy-tools"
)

// protectedRepositoriesDocument は、テストの保護対象の一覧である。
// 許可 Caller のある保護対象 app と、許可 Caller のない保護対象 vault を持つ。
// 許可 Caller の権限の上限は、発行上限より高い水準（pull_requests:write）を含み、発行上限が含む組（issues:write）を含まない。
const protectedRepositoriesDocument = `repositories:
  - id: "200"
    name: app
    callers:
      - repository_id: "3003"
        repository_name: deploy-tools
        permissions:
          contents: write
          pull_requests: write
  - id: "201"
    name: vault
    callers: []
`

var contentsRead = map[string]string{"contents": "read"}

// issuablePermissionNotations は、テストの発行上限である。同じ権限名の複数の水準（`contents`）、高い水準だけを含む権限名（`issues:write`）、
// 低い水準だけを含む権限名（`pull_requests:read`）、付随権限の組を含む。
var issuablePermissionNotations = []string{"contents:read", "contents:write", "issues:write", "pull_requests:read", "metadata:read"}

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
	c, err := caller.NewCaller(ownerID, callerRepositoryID, callerRepositoryName, 4004, 1, caller.WorkflowTriggerAttributes{})
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	return c
}

// mustNewCallerOf は、許可オーナーのリポジトリから呼び出した Caller を、リポジトリを変えて作る。保護対象の許可 Caller との一致を変えるため。
func mustNewCallerOf(t *testing.T, repositoryID int64, repositoryName string) caller.Caller {
	t.Helper()
	c, err := caller.NewCaller(allowedOwnerID, repositoryID, repositoryName, 4004, 1, caller.WorkflowTriggerAttributes{})
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	return c
}

func mustParseProtectedRepositories(t *testing.T, document string) protected.Repositories {
	t.Helper()
	r, err := protected.Parse(document)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	return r
}

// noProtectedRepositories は、保護対象のない一覧である。保護対象の条件の外の規則を確かめるテストが使う。
func noProtectedRepositories(t *testing.T) protected.Repositories {
	t.Helper()
	return mustParseProtectedRepositories(t, "repositories: []\n")
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

// repositoriesOf は、許可オーナーの n 件の対象の完全名を作る。対象の件数の上限の前後を確かめるため。
func repositoriesOf(n int) []string {
	repositories := make([]string, n)
	for i := range repositories {
		repositories[i] = fmt.Sprintf("Example-Org/repo%d", i)
	}
	return repositories
}

// repositoryPolicyAllowing は、許可 Caller を 1 つだけ持つリポジトリポリシーを取得できた結果を作る。
// 権限名を並べ替えて書くのは、map の順序によらず同じ文書にするため。
func repositoryPolicyAllowing(repositoryID int64, repositoryName string, permissions map[string]string) repositorypolicy.FetchResult {
	var b strings.Builder
	fmt.Fprintf(&b, "callers:\n  - repository_id: \"%d\"\n    repository_name: %s\n    permissions:\n", repositoryID, repositoryName)
	for _, name := range slices.Sorted(maps.Keys(permissions)) {
		fmt.Fprintf(&b, "      %s: %s\n", name, permissions[name])
	}
	return repositorypolicy.Document(b.String())
}

// repeatFetchResult は、n 件の対象のすべてに同じ取得の結果を並べる。
func repeatFetchResult(n int, result repositorypolicy.FetchResult) []repositorypolicy.FetchResult {
	results := make([]repositorypolicy.FetchResult, n)
	for i := range results {
		results[i] = result
	}
	return results
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
