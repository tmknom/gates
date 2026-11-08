package handler

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/request"
	"github.com/tmknom/gates/internal/tokenissuer"
)

const (
	allowedOwnerID   = 583231
	allowedOwnerName = "allowed-owner"
	installationID   = 12345678

	// tokenMarker は、記録した発行応答のトークンで、成功応答に値がそのまま入ることを確かめる目印である。
	tokenMarker = "ghs_token-marker" //nolint:gosec // 資格情報ではなく、値を確かめるための目印である
	// idTokenMarker は、Payload の GATES 用 OIDC JWT で、応答と監査記録に現れないことを確かめる目印である。
	idTokenMarker = "id-token-header.id-token-payload.id-token-signature"
)

// now は、記録した発行応答の有効期限の 1 時間前である。
var now = time.Date(2026, 11, 3, 11, 18, 48, 0, time.UTC)

// signerStub は KMS Signer のテストダブルである。
type signerStub struct{}

func (signerStub) Sign(context.Context) (github.AppJWT, error) {
	return github.NewAppJWT("app-jwt"), nil
}

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

// issueInstallationToken は、返却可能な発行結果を Token Issuer の照合からしか作れないため、GitHub API の代わりのサーバーに対して Token Issuer を呼ぶ。
func issueInstallationToken(t *testing.T, issuance map[string]any, revocationStatus int) (tokenissuer.InstallationToken, error) {
	t.Helper()
	owner := allowedOwner(t)
	return newTokenIssuer(t, issuance, revocationStatus).IssueInstallationToken(t.Context(), func(installation authz.Installation) (authz.TokenGrant, error) {
		return authz.Grant(verifiedCaller(t), tokenRequest(t), owner, installation)
	})
}

// newTokenIssuer は、GitHub API の代わりのサーバーに接続する Token Issuer を作る。
// サーバーは許可オーナーの停止していない Installation を返し、issuance を発行応答として返し、失効の要求に revocationStatus で応答する。
func newTokenIssuer(t *testing.T, issuance map[string]any, revocationStatus int) *tokenissuer.TokenIssuer {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/"+allowedOwnerName+"/installation", respond(t, http.StatusOK, map[string]any{"id": installationID, "account": map[string]any{"id": allowedOwnerID}, "suspended_at": nil}))
	mux.HandleFunc("POST /app/installations/12345678/access_tokens", respond(t, http.StatusCreated, issuance))
	mux.HandleFunc("DELETE /installation/token", respond(t, revocationStatus, nil))
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	apiHost, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	client := github.NewClient(github.WithAPIHost(apiHost), github.WithTransport(server.Client().Transport))
	return tokenissuer.NewTokenIssuer(signerStub{}, client, allowedOwner(t), func() time.Time { return now })
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

func verifiedCaller(t *testing.T) caller.Caller {
	t.Helper()
	return callerOf(t, allowedOwnerID)
}

func callerOf(t *testing.T, ownerID int64) caller.Caller {
	t.Helper()
	c, err := caller.NewCaller(ownerID, 1234567890, 4004, 1, caller.WorkflowTriggerAttributes{})
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
		"id_token":     idTokenMarker,
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
