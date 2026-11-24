package handler

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json/v2"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/repositorypolicy"
	"github.com/tmknom/gates/internal/request"
	"github.com/tmknom/gates/internal/tokenissuer"
)

const (
	allowedOwnerID   = 583231
	allowedOwnerName = "allowed-owner"
	installationID   = 12345678

	// tokenMarker は、記録した発行応答のトークンで、成功応答に値がそのまま入ることを確かめる目印である。
	tokenMarker = "ghs_token-marker" //nolint:gosec // 資格情報ではなく、値を確かめるための目印である
	// oidcJWTMarker は、Payload の GATES 用 OIDC JWT で、応答と監査記録に現れないことを確かめる目印である。
	oidcJWTMarker = "id-token-header.id-token-payload.id-token-signature"
	// claimsSetMarker は、署名検証済みの Claims Set の Claim の値で、監査記録にだけ現れ、応答に現れないことを確かめる目印である。
	claimsSetMarker = "claims-set-marker"
	// policyReadTokenMarker は、ポリシー読み取りトークンで、応答と監査記録に現れないことを確かめる目印である。
	policyReadTokenMarker = "ghs_policy-read-token-marker" //nolint:gosec // 資格情報ではなく、値を確かめるための目印である
	// repositoryPolicyMarker、allowedCallerNameMarker、allowedCallerIDMarker は、リポジトリポリシーの内容で、応答と監査記録に現れないことを確かめる目印である。
	repositoryPolicyMarker  = "repository-policy-marker"
	allowedCallerNameMarker = "allowed-caller-marker"
	allowedCallerIDMarker   = "987654321098"
)

// lambdaRequestID は、Lambda の request ID の形をした相関 ID である。
const lambdaRequestID = "c6af9ac6-7b61-11e6-9a41-93e8deadbeef"

// now は、記録した発行応答の有効期限の 1 時間前である。
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
			map[string]any{"id": 1234567890, "full_name": allowedOwnerName + "/example-repo", "owner": map[string]any{"id": allowedOwnerID}},
			map[string]any{"id": 1234567891, "full_name": allowedOwnerName + "/repo-b", "owner": map[string]any{"id": allowedOwnerID}},
		},
	}
}

// issueInstallationAccessToken は、返却可能な発行結果を Token Issuer の照合からしか作れないため、GitHub API の代わりのサーバーに対して Token Issuer を呼ぶ。
func issueInstallationAccessToken(t *testing.T, issuance map[string]any, revocationStatus int) (tokenissuer.ReturnableIssuanceResult, error) {
	t.Helper()
	returnable, _, err := newTokenIssuer(t, issuance, revocationStatus).IssueInstallationAccessToken(t.Context(), notEnforcedAuthorization{t: t})
	return returnable, err
}

// notEnforcedAuthorization は、リポジトリポリシーを適用しないデプロイの Authorization で、Handler を通さずに Token Issuer を呼ぶテストが使う。
type notEnforcedAuthorization struct {
	t *testing.T
}

func (a notEnforcedAuthorization) GrantPolicyRead(authz.Installation) (authz.PolicyReadGrant, error) {
	a.t.Error("GrantPolicyRead was called without repository policy enforcement")
	return authz.PolicyReadGrant{}, nil
}

func (a notEnforcedAuthorization) GrantWithRepositoryPolicies(authz.Installation, []repositorypolicy.FetchResult) (authz.TokenGrant, error) {
	a.t.Error("GrantWithRepositoryPolicies was called without repository policy enforcement")
	return authz.TokenGrant{}, nil
}

func (a notEnforcedAuthorization) Grant(installation authz.Installation) (authz.TokenGrant, error) {
	return authz.Grant(verifiedCaller(a.t), tokenRequest(a.t), allowedOwner(a.t), mustNewIssuablePermissions(a.t, "contents:read"), noProtectedRepositories(a.t), authz.RepositoryPolicyNotEnforced, installation)
}

// signerStub は KMS Signer のテストダブルである。
type signerStub struct{}

func (signerStub) Sign(context.Context) (github.AppJWT, error) {
	return github.NewAppJWT("app-jwt"), nil
}

// newTokenIssuer は、GitHub API の代わりのサーバーに接続する Token Issuer を作る。
// サーバーは許可オーナーの停止していない Installation を返し、issuance を発行応答として返し、失効の要求に revocationStatus で応答する。
func newTokenIssuer(t *testing.T, issuance map[string]any, revocationStatus int) *tokenissuer.TokenIssuer {
	t.Helper()
	tokenIssuer, _ := newTokenIssuerRecordingIssuance(t, noProtectedRepositories(t), issuance, revocationStatus)
	return tokenIssuer
}

// newTokenIssuerRecordingIssuance は、newTokenIssuer と同じ Token Issuer を、保護対象の一覧を指定して作り、発行 API が受け取った本文の記録を返す。
// サーバーのハンドラは別の goroutine で動くため、記録を atomic.Pointer で持つ。
func newTokenIssuerRecordingIssuance(t *testing.T, protectedRepositories protected.Repositories, issuance map[string]any, revocationStatus int) (*tokenissuer.TokenIssuer, *atomic.Pointer[[]byte]) {
	t.Helper()
	var issuanceBody atomic.Pointer[[]byte]
	respondIssuance := respond(t, http.StatusCreated, issuance)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/"+allowedOwnerName+"/installation", respond(t, http.StatusOK, map[string]any{"id": installationID, "account": map[string]any{"id": allowedOwnerID}, "suspended_at": nil}))
	mux.HandleFunc("POST /app/installations/12345678/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the issuance request body returned error: %v", err)
		}
		issuanceBody.Store(&b)
		respondIssuance(w, r)
	})
	mux.HandleFunc("DELETE /installation/token", respond(t, revocationStatus, nil))
	client := newGitHubClient(t, mux)
	return tokenissuer.NewTokenIssuer(signerStub{}, client, allowedOwner(t), protectedRepositories, authz.RepositoryPolicyNotEnforced, func() time.Time { return now }), &issuanceBody
}

// enforcedGitHub は、リポジトリポリシーを適用するデプロイの Token Issuer が呼ぶ GitHub API の代わりのサーバーの応答である。
// 発行の経路は、要求の本文が single_file の権限を含めばポリシー読み取りトークンの発行、含まなければ利用者に返すトークンの発行として応答する。
type enforcedGitHub struct {
	policyReadIssuance map[string]any
	issuance           map[string]any
	// policies は、リポジトリ名を小文字にした値から、Contents API の応答への対応である
	policies map[string]http.HandlerFunc
}

// newEnforcedTokenIssuer は、リポジトリポリシーを適用する Token Issuer を作り、利用者に返すトークンの発行 API が受け取った本文の記録を返す。
// 失効の要求には、どちらのトークンにも 204 で応答する。
func newEnforcedTokenIssuer(t *testing.T, protectedRepositories protected.Repositories, gitHub enforcedGitHub) (*tokenissuer.TokenIssuer, *atomic.Pointer[[]byte]) {
	t.Helper()
	var issuanceBody atomic.Pointer[[]byte]
	respondPolicyReadIssuance := respond(t, http.StatusCreated, gitHub.policyReadIssuance)
	respondIssuance := respond(t, http.StatusCreated, gitHub.issuance)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/"+allowedOwnerName+"/installation", respond(t, http.StatusOK, map[string]any{"id": installationID, "account": map[string]any{"id": allowedOwnerID}, "suspended_at": nil}))
	mux.HandleFunc("POST /app/installations/12345678/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the issuance request body returned error: %v", err)
		}
		if bytes.Contains(b, []byte(`"single_file"`)) {
			respondPolicyReadIssuance(w, r)
			return
		}
		issuanceBody.Store(&b)
		respondIssuance(w, r)
	})
	mux.HandleFunc("GET /repos/"+allowedOwnerName+"/{repo}/contents/.github/gates-policy.yml", func(w http.ResponseWriter, r *http.Request) {
		policy, ok := gitHub.policies[strings.ToLower(r.PathValue("repo"))]
		if !ok {
			t.Errorf("repository policy of %q was read, but the test has no response for it", r.PathValue("repo"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		policy(w, r)
	})
	mux.HandleFunc("DELETE /installation/token", respond(t, http.StatusNoContent, nil))
	client := newGitHubClient(t, mux)
	return tokenissuer.NewTokenIssuer(signerStub{}, client, allowedOwner(t), protectedRepositories, authz.RepositoryPolicyEnforced, func() time.Time { return now }), &issuanceBody
}

// recordedPolicyReadIssuance は、ポリシー読み取りトークンの発行応答の形で、実効対象を recordedIssuance と同じにする。
func recordedPolicyReadIssuance() map[string]any {
	body := recordedIssuance()
	body["token"] = policyReadTokenMarker
	body["permissions"] = map[string]any{"single_file": "read", "metadata": "read"}
	body["single_file"] = ".github/gates-policy.yml"
	body["has_multiple_single_files"] = false
	body["single_file_paths"] = []any{".github/gates-policy.yml"}
	return body
}

// repositoryPolicyFile は、Contents API のファイルの応答の形で document を返す。
func repositoryPolicyFile(t *testing.T, document string) http.HandlerFunc {
	t.Helper()
	return respond(t, http.StatusOK, map[string]any{
		"type":     "file",
		"encoding": "base64",
		"size":     len(document),
		"name":     "gates-policy.yml",
		"path":     ".github/gates-policy.yml",
		"content":  base64.StdEncoding.EncodeToString([]byte(document)),
	})
}

// allowingRepositoryPolicy は、Caller に contents:read までを許すリポジトリポリシーの文書である。
// 応答とログに内容が出ないことを確かめるため、ほかの許可 Caller とコメントに目印を置く。
func allowingRepositoryPolicy() string {
	return "callers:\n" +
		"  - repository_id: \"1234567890\"\n    repository_name: example-repo\n    permissions:\n      contents: read\n" +
		"  - repository_id: \"" + allowedCallerIDMarker + "\"\n    repository_name: " + allowedCallerNameMarker + "\n    permissions:\n      contents: read\n" +
		"# " + repositoryPolicyMarker + "\n"
}

// denyingRepositoryPolicy は、Caller を許さないリポジトリポリシーの文書である。
func denyingRepositoryPolicy() string {
	return "callers:\n" +
		"  - repository_id: \"" + allowedCallerIDMarker + "\"\n    repository_name: " + allowedCallerNameMarker + "\n    permissions:\n      contents: read\n" +
		"# " + repositoryPolicyMarker + "\n"
}

// newGitHubClient は、mux で応答するサーバーに接続する GitHub Client を作る。
func newGitHubClient(t *testing.T, mux *http.ServeMux) *github.Client {
	t.Helper()
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	apiHost, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(server.Certificate())
	return github.NewClient(github.WithAPIHost(apiHost), github.WithRootCAs(rootCAs))
}

func respond(t *testing.T, status int, body any) http.HandlerFunc {
	t.Helper()
	var b []byte
	if body != nil {
		b = marshal(t, body)
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(b)
	}
}

func marshal(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return b
}

func allowedOwner(t *testing.T) authz.AllowedOwner {
	t.Helper()
	owner, err := authz.NewAllowedOwner(allowedOwnerID, allowedOwnerName, authz.OwnerTypeUser)
	if err != nil {
		t.Fatalf("NewAllowedOwner returned error: %v", err)
	}
	return owner
}

func mustNewIssuablePermissions(t *testing.T, notations ...string) permission.IssuablePermissions {
	t.Helper()
	p, err := permission.NewIssuablePermissions(notations)
	if err != nil {
		t.Fatalf("NewIssuablePermissions(%v) returned error: %v", notations, err)
	}
	return p
}

func mustParseProtectedRepositories(t *testing.T, document string) protected.Repositories {
	t.Helper()
	r, err := protected.Parse(document)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	return r
}

// noProtectedRepositories は、保護対象のない一覧である。保護対象のない一覧のデプロイで、既存の要求の結果が変わらないことを確かめるため、既定にする。
func noProtectedRepositories(t *testing.T) protected.Repositories {
	t.Helper()
	return mustParseProtectedRepositories(t, "repositories: []\n")
}

func verifiedCaller(t *testing.T) caller.Caller {
	t.Helper()
	return callerOf(t, allowedOwnerID)
}

func callerOf(t *testing.T, ownerID int64) caller.Caller {
	t.Helper()
	c, err := caller.NewCaller(ownerID, 1234567890, "example-repo", 4004, 1, caller.WorkflowTriggerAttributes{})
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	return c
}

// validPayload は、記録した発行応答の実効対象と一致する対象を要求する Payload である。
func validPayload(t *testing.T) []byte {
	t.Helper()
	return marshal(t, map[string]any{
		"version":      1,
		"id_token":     oidcJWTMarker,
		"repositories": []string{allowedOwnerName + "/repo-b", allowedOwnerName + "/Example-Repo"},
		"permissions":  map[string]string{"contents": "read"},
	})
}

func tokenRequest(t *testing.T) request.TokenRequest {
	t.Helper()
	_, req, err := request.Validate(validPayload(t))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return req
}

func decodeResponse(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("response %s is not a JSON object: %v", b, err)
	}
	return decoded
}

func assertKeys(t *testing.T, name string, object map[string]any, want ...string) {
	t.Helper()
	if got := slices.Sorted(maps.Keys(object)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Errorf("%s keys = %q, want %q", name, got, want)
	}
}
