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
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/request"
)

const (
	allowedOwnerID   = 583231
	allowedOwnerName = "allowed-owner"
	installationID   = 12345678
	requestID        = "ABCD:1234:5678:9ABC:DEF0"
	repositoryIDA    = 1234567890
	repositoryIDB    = 1234567891

	// tokenMarker は、トークンが出力に漏れていないことを確かめる目印である。
	tokenMarker = "ghs_token-marker" //nolint:gosec // 資格情報ではなく、漏れを探すための目印である
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
	client := github.NewClient(github.WithAPIHost(apiHost), github.WithRootCAs(rootCAs))
	result, err := client.Issue(t.Context(), github.NewAppJWT("app-jwt"), tokenGrant(t, contentsRead))
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	return result
}

// tokenGrant は、TokenGrant を Grant でしか作れないため、記録した Installation で grant を通す。
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
// 発行上限は、Grant が認可を判定し直しても通るよう、要求の Permissions と同じ組にする。
func grant(t *testing.T, installation authz.Installation, permissions map[string]string) (authz.TokenGrant, error) {
	t.Helper()
	c, err := caller.NewCaller(allowedOwnerID, repositoryIDA, 4004, 1, caller.WorkflowTriggerAttributes{})
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	payload := marshal(t, map[string]any{
		"version":      1,
		"id_token":     "header.payload.signature",
		"repositories": requestedRepositories,
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
	return authz.Grant(c, req, allowedOwner(t), issuablePermissions, installation)
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
	return verify(result, tokenGrant(t, granted), allowedOwner(t), now)
}
