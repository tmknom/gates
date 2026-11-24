package tokenissuer

import (
	"crypto/x509"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/request"
)

const (
	allowedOwnerID   = 583231
	allowedOwnerName = "allowed-owner"
	installationID   = 12345678
	requestID        = "ABCD:1234:5678:9ABC:DEF0"
	repositoryIDA    = 1234567890
	repositoryIDB    = 1234567891

	// responseMarker は、応答本文がエラー文言に漏れていないことを確かめる目印である。
	responseMarker = "response-body-marker"

	// tokenMarker は、トークンが出力に漏れていないことを確かめる目印である。
	tokenMarker = "ghs_token-marker" //nolint:gosec // 資格情報ではなく、漏れを探すための目印である

	// policyReadTokenMarker は、Token Issuer の公開操作のテストで、ポリシー読み取りトークンを利用者に返すトークンと見分ける目印である。
	policyReadTokenMarker = "ghs_policy-read-token-marker" //nolint:gosec // 資格情報ではなく、漏れを探すための目印である
)

// requestedRepositories は、発行許可の対象である。大文字小文字だけが異なる実効対象を受理することを確かめるため、大文字を含める。
var requestedRepositories = []string{allowedOwnerName + "/repo-b", allowedOwnerName + "/Example-Repo"}

// contentsRead は、調査記録の発行応答を得た要求の Permissions である。
var contentsRead = map[string]string{"contents": "read"}

// now は、記録した応答の有効期限の 1 時間前である。
var now = time.Date(2026, 11, 3, 11, 18, 48, 0, time.UTC)

// recordedIssuance は、調査記録の発行応答の形を、ケースごとに書き換えられる形で返す。
// 実効対象は要求と異なる順序で、一方は要求と大文字小文字だけが異なる表記にする。
func recordedIssuance() map[string]any {
	return map[string]any{
		"token":                tokenMarker,
		"expires_at":           "2026-11-03T12:18:48Z",
		"permissions":          map[string]any{"contents": "read", "metadata": "read"},
		"repository_selection": "selected",
		"repositories": []any{
			effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/example-repo"),
			effectiveRepository(repositoryIDB, allowedOwnerID, allowedOwnerName+"/repo-b"),
		},
	}
}

// recordedPolicyReadIssuance は、ポリシー読み取りトークンの発行応答の形を、ケースごとに書き換えられる形で返す。
// 調査記録は Single file のパスを README.md にした発行応答で、パスの値だけをリポジトリポリシーのパスに置き換えている。
func recordedPolicyReadIssuance() map[string]any {
	body := recordedIssuance()
	body["permissions"] = map[string]any{"single_file": "read", "metadata": "read"}
	body["single_file"] = ".github/gates-policy.yml"
	body["has_multiple_single_files"] = false
	body["single_file_paths"] = []any{".github/gates-policy.yml"}
	return body
}

func effectiveRepository(id, ownerID int64, fullName string) map[string]any {
	_, name, _ := strings.Cut(fullName, "/")
	return map[string]any{
		"id":        id,
		"name":      name,
		"full_name": fullName,
		"private":   true,
		"owner":     map[string]any{"login": allowedOwnerName, "id": ownerID, "type": "User"},
	}
}

func marshal(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return string(b)
}

// issue は、発行結果を GitHub Client の発行応答の読み取りからしか作れないため、body を 201 で返すサーバーに対して Issue を呼ぶ。
func issue(t *testing.T, body string) github.IssuanceResult {
	t.Helper()
	result, err := issuanceClient(t, body).Issue(t.Context(), github.NewAppJWT("app-jwt"), tokenGrant(t, contentsRead))
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	return result
}

// issuePolicyRead は、issue と同じく、body を 201 で返すサーバーに対して IssuePolicyReadToken を呼ぶ。
func issuePolicyRead(t *testing.T, body string) github.IssuanceResult {
	t.Helper()
	result, err := issuanceClient(t, body).IssuePolicyReadToken(t.Context(), github.NewAppJWT("app-jwt"), policyReadGrant(t))
	if err != nil {
		t.Fatalf("IssuePolicyReadToken returned error: %v", err)
	}
	return result
}

// issuanceClient は、どの要求にも body を 201 で返すサーバーに向けた GitHub Client を作る。
func issuanceClient(t *testing.T, body string) *github.Client {
	t.Helper()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", requestID)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, body)
	}))
	server.StartTLS()
	t.Cleanup(server.Close)
	apiHost, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(server.Certificate())
	return github.NewClient(github.WithAPIHost(apiHost), github.WithRootCAs(rootCAs))
}

// tokenGrant は、TokenGrant を authz の確定関数でしか作れないため、記録した Installation で grant を通す。
func tokenGrant(t *testing.T, permissions map[string]string) authz.TokenGrant {
	t.Helper()
	installation, err := authz.NewInstallation(installationID, allowedOwnerID, false)
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	tokenGrant, err := grant(t, installation, permissions)
	if err != nil {
		t.Fatalf("Grant returned error: %v", err)
	}
	return tokenGrant
}

// grant は、Lambda Handler と同じく、Caller、形式検証を通した発行要求、許可オーナー、発行上限をそろえて、Installation から発行許可を確定する。
func grant(t *testing.T, installation authz.Installation, permissions map[string]string) (authz.TokenGrant, error) {
	t.Helper()
	inputs := newGrantInputs(t, permissions)
	return authz.Grant(inputs.caller, inputs.request, allowedOwner(t), inputs.issuablePermissions, noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced, installation)
}

// policyReadGrant は、requestedRepositories を対象とするポリシー読み取り許可を作る。
func policyReadGrant(t *testing.T) authz.PolicyReadGrant {
	t.Helper()
	return policyReadGrantOf(t, requestedRepositories)
}

// policyReadGrantOf は、PolicyReadGrant を GrantPolicyRead でしか作れないため、記録した Installation で GrantPolicyRead を通す。
func policyReadGrantOf(t *testing.T, repositories []string) authz.PolicyReadGrant {
	t.Helper()
	installation, err := authz.NewInstallation(installationID, allowedOwnerID, false)
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	inputs := newGrantInputsOf(t, repositories, contentsRead)
	grant, err := authz.GrantPolicyRead(inputs.caller, inputs.request, allowedOwner(t), inputs.issuablePermissions, noProtectedRepositories(t), authz.RepositoryPolicyEnforced, installation)
	if err != nil {
		t.Fatalf("GrantPolicyRead returned error: %v", err)
	}
	return grant
}

// grantInputs は、発行許可とポリシー読み取り許可の確定に渡す Caller、形式検証を通した発行要求、発行上限である。
type grantInputs struct {
	caller              caller.Caller
	request             request.TokenRequest
	issuablePermissions permission.IssuablePermissions
}

// newGrantInputs は、requestedRepositories を対象とする要求の値をそろえる。
func newGrantInputs(t *testing.T, permissions map[string]string) grantInputs {
	t.Helper()
	return newGrantInputsOf(t, requestedRepositories, permissions)
}

// newGrantInputsOf は、repositories を対象とする要求の値をそろえる。
// 発行上限は、認可を判定し直しても通るよう、要求の Permissions と同じ組にする。
func newGrantInputsOf(t *testing.T, repositories []string, permissions map[string]string) grantInputs {
	t.Helper()
	c, err := caller.NewCaller(allowedOwnerID, repositoryIDA, "example-repo", 4004, 1, caller.WorkflowTriggerAttributes{})
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	payload := marshal(t, map[string]any{
		"version":      1,
		"id_token":     "header.payload.signature",
		"repositories": repositories,
		"permissions":  permissions,
	})
	_, req, err := request.Validate([]byte(payload))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	notations := make([]string, 0, len(permissions))
	for name, level := range permissions {
		notations = append(notations, name+":"+level)
	}
	issuablePermissions, err := permission.NewIssuablePermissions(notations)
	if err != nil {
		t.Fatalf("NewIssuablePermissions returned error: %v", err)
	}
	return grantInputs{caller: c, request: req, issuablePermissions: issuablePermissions}
}

func allowedOwner(t *testing.T) authz.AllowedOwner {
	t.Helper()
	owner, err := authz.NewAllowedOwner(allowedOwnerID, allowedOwnerName, authz.OwnerTypeUser)
	if err != nil {
		t.Fatalf("NewAllowedOwner returned error: %v", err)
	}
	return owner
}

func verifyResult(t *testing.T, result github.IssuanceResult, granted map[string]string) (ReturnableIssuanceResult, error) {
	t.Helper()
	return verify(result, tokenGrant(t, granted), allowedOwner(t), noProtectedRepositories(t), now)
}

func mustParseProtectedRepositories(t *testing.T, document string) protected.Repositories {
	t.Helper()
	r, err := protected.Parse(document)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	return r
}

// noProtectedRepositories は、保護対象のない一覧である。保護対象との対応の外の規則を確かめるテストが使う。
func noProtectedRepositories(t *testing.T) protected.Repositories {
	t.Helper()
	return mustParseProtectedRepositories(t, "repositories: []\n")
}

// protectedRepositoriesOf は、許可 Caller のない保護対象を 1 つ持つ一覧を作る。発行後の照合は許可 Caller を見ないため。
func protectedRepositoriesOf(t *testing.T, id, name string) protected.Repositories {
	t.Helper()
	return mustParseProtectedRepositories(t, "repositories:\n  - id: \""+id+"\"\n    name: "+name+"\n    callers: []\n")
}

// newTestTokenIssuer は、リポジトリポリシーを適用しないデプロイの Token Issuer を作る。
func newTestTokenIssuer(t *testing.T, signer *signerStub, stub *gitHubStub) *TokenIssuer {
	t.Helper()
	return NewTokenIssuer(signer, stub.client(t), allowedOwner(t), noProtectedRepositories(t), authz.RepositoryPolicyNotEnforced, func() time.Time { return now })
}

// newEnforcedTestTokenIssuer は、リポジトリポリシーを適用するデプロイの Token Issuer を作る。
func newEnforcedTestTokenIssuer(t *testing.T, signer *signerStub, stub *gitHubStub) *TokenIssuer {
	t.Helper()
	return NewTokenIssuer(signer, stub.client(t), allowedOwner(t), noProtectedRepositories(t), authz.RepositoryPolicyEnforced, func() time.Time { return now })
}

// assertHidesValues は、エラー文言に GitHub App JWT、トークン、ポリシー読み取りトークン、応答本文が入らないことを確かめる。
func assertHidesValues(t *testing.T, err error) {
	t.Helper()
	for _, value := range []string{appJWTMarker, tokenMarker, policyReadTokenMarker, responseMarker} {
		if strings.Contains(err.Error(), value) {
			t.Errorf("error message %q contains %q", err.Error(), value)
		}
	}
}

func mustParseFullName(t *testing.T, s string) repository.FullName {
	t.Helper()
	fullName, err := repository.ParseFullName(s)
	if err != nil {
		t.Fatalf("ParseFullName returned error: %v", err)
	}
	return fullName
}

func mustRepositoryID(t *testing.T, id int64) githubid.RepositoryID {
	t.Helper()
	repositoryID, err := githubid.NewRepositoryID(id)
	if err != nil {
		t.Fatalf("NewRepositoryID returned error: %v", err)
	}
	return repositoryID
}
