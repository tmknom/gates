package tokenissuer

import (
	"crypto/x509"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
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
	otherOwnerID     = 9876543
	installationID   = 12345678
	requestID        = "ABCD:1234:5678:9ABC:DEF0"
	repositoryIDA    = 1234567890
	repositoryIDB    = 1234567891

	// tokenMarker は、トークンが出力に漏れていないことを確かめる目印である。
	tokenMarker = "ghs_token-marker" //nolint:gosec // 資格情報ではなく、漏れを探すための目印である
)

// requestedRepositories は、発行許可の対象である。大文字小文字だけが異なる実効対象を受理することを確かめるため、大文字を含める。
var requestedRepositories = []string{allowedOwnerName + "/repo-b", allowedOwnerName + "/Example-Repo"}

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
	result, err := client.Issue(t.Context(), github.NewAppJWT("app-jwt"), tokenGrant(t))
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	return result
}

// tokenGrant は、TokenGrant を Grant でしか作れないため、記録した Installation で grant を通す。
func tokenGrant(t *testing.T) authz.TokenGrant {
	t.Helper()
	installation, err := authz.NewInstallation(installationID, allowedOwnerID, time.Time{})
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	tokenGrant, err := grant(t, installation)
	if err != nil {
		t.Fatalf("Grant returned error: %v", err)
	}
	return tokenGrant
}

// grant は、Lambda Handler と同じく、Caller、形式検証を通した発行要求、許可オーナーをそろえて、Installation から発行許可を確定する。
func grant(t *testing.T, installation authz.Installation) (authz.TokenGrant, error) {
	t.Helper()
	c, err := caller.NewCaller(allowedOwnerID, repositoryIDA, 4004, 1, caller.WorkflowTriggerAttributes{})
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	payload := marshal(t, map[string]any{
		"version":      1,
		"id_token":     "header.payload.signature",
		"repositories": requestedRepositories,
		"permissions":  map[string]string{"contents": "read"},
	})
	_, req, err := request.Validate([]byte(payload))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return authz.Grant(c, req, allowedOwner(t), installation)
}

func allowedOwner(t *testing.T) authz.AllowedOwner {
	t.Helper()
	owner, err := authz.NewAllowedOwner(allowedOwnerID, allowedOwnerName, authz.OwnerTypeUser)
	if err != nil {
		t.Fatalf("NewAllowedOwner returned error: %v", err)
	}
	return owner
}

func verifyResult(t *testing.T, result github.IssuanceResult) (InstallationToken, error) {
	t.Helper()
	return verify(result, tokenGrant(t), allowedOwner(t), now)
}

func mustNewPermissions(t *testing.T, levels map[string]string) permission.Permissions {
	t.Helper()
	p, err := permission.NewPermissions(levels)
	if err != nil {
		t.Fatalf("NewPermissions returned error: %v", err)
	}
	return p
}

func fullNames(repositories []github.EffectiveRepository) []string {
	names := make([]string, 0, len(repositories))
	for _, r := range repositories {
		names = append(names, r.FullName().String())
	}
	return names
}

func TestVerify_ReturnsInstallationTokenOfIssuanceResult(t *testing.T) {
	result := issue(t, marshal(t, recordedIssuance()))

	got, err := verifyResult(t, result)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if token, _ := result.Token(); got.Token() != token {
		t.Errorf("Token() is not the token of the issuance result")
	}
	if want := time.Date(2026, 11, 3, 12, 18, 48, 0, time.UTC); !got.ExpiresAt().Equal(want) {
		t.Errorf("ExpiresAt() = %v, want %v", got.ExpiresAt(), want)
	}
	if want := mustNewPermissions(t, map[string]string{"contents": "read", "metadata": "read"}); !got.EffectivePermissions().Equal(want) {
		t.Errorf("EffectivePermissions() is not equal to {contents:read, metadata:read}")
	}
	// 成功応答の repositories は GitHub 応答の full_name で示すため、要求の表記ではなく応答の表記と順序のまま持つ
	wantNames := []string{allowedOwnerName + "/example-repo", allowedOwnerName + "/repo-b"}
	if names := fullNames(got.EffectiveRepositories()); !reflect.DeepEqual(names, wantNames) {
		t.Errorf("EffectiveRepositories() full names = %v, want %v", names, wantNames)
	}
	if ids := []int64{got.EffectiveRepositories()[0].RepositoryID(), got.EffectiveRepositories()[1].RepositoryID()}; !reflect.DeepEqual(ids, []int64{repositoryIDA, repositoryIDB}) {
		t.Errorf("EffectiveRepositories() ids = %v, want %v", ids, []int64{repositoryIDA, repositoryIDB})
	}
}

func TestVerify_Accepts(t *testing.T) {
	tests := []struct {
		name   string
		modify func(response map[string]any)
	}{
		{name: "allowed permissions without implied permissions are accepted", modify: func(response map[string]any) {
			response["permissions"] = map[string]any{"contents": "read"}
		}},
		{name: "token of 520 characters is accepted", modify: func(response map[string]any) {
			response["token"] = "ghs_" + strings.Repeat("a", 516)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := recordedIssuance()
			tt.modify(response)
			if _, err := verifyResult(t, issue(t, marshal(t, response))); err != nil {
				t.Errorf("Verify returned error: %v", err)
			}
		})
	}
}

//trace:checks AC-06 AC-11 AC-12
func TestVerify_Rejects(t *testing.T) {
	tests := []struct {
		name string
		body func(t *testing.T) string
		// condition は、満たさなかった条件として文言に書かれる語である。ほかの条件で拒否されていないことを確かめる
		condition string
	}{
		{name: "body that is not JSON returns ErrVerificationFailure", body: func(*testing.T) string { return "not json" }, condition: "token is unreadable"},
		{name: "empty token returns ErrVerificationFailure", body: modified(func(r map[string]any) { r["token"] = "" }), condition: "token is unreadable"},
		{name: "missing expires_at returns ErrVerificationFailure", body: modified(func(r map[string]any) { delete(r, "expires_at") }), condition: "expires_at is unreadable"},
		{name: "expires_at equal to now returns ErrVerificationFailure", body: modified(func(r map[string]any) { r["expires_at"] = now.Format(time.RFC3339) }), condition: "expires_at is not after now"},
		{name: "expires_at before now returns ErrVerificationFailure", body: modified(func(r map[string]any) { r["expires_at"] = now.Add(-time.Second).Format(time.RFC3339) }), condition: "expires_at is not after now"},
		{name: "missing repositories returns ErrVerificationFailure", body: modified(func(r map[string]any) { delete(r, "repositories") }), condition: "repositories are unreadable"},
		{name: "full_name that is not a full name returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDA, allowedOwnerID, "example-repo"), effectiveRepository(repositoryIDB, allowedOwnerID, allowedOwnerName+"/repo-b")}
		}), condition: "repositories are unreadable"},
		{name: "missing repository returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDB, allowedOwnerID, allowedOwnerName+"/repo-b")}
		}), condition: "repositories count does not match token grant"},
		{name: "additional repository returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["repositories"] = append(r["repositories"].([]any), effectiveRepository(1234567892, allowedOwnerID, allowedOwnerName+"/repo-c"))
		}), condition: "repositories count does not match token grant"},
		{name: "repository outside token grant returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/example-repo"), effectiveRepository(repositoryIDB, allowedOwnerID, allowedOwnerName+"/repo-c")}
		}), condition: "repositories[1] full_name does not match token grant"},
		{name: "duplicate full_name returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDB, allowedOwnerID, allowedOwnerName+"/repo-b"), effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/Repo-B")}
		}), condition: "repositories[1] full_name does not match token grant"},
		{name: "duplicate repository id returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/example-repo"), effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/repo-b")}
		}), condition: "repositories[1] id is a duplicate"},
		{name: "repository id of 0 returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/example-repo"), effectiveRepository(0, allowedOwnerID, allowedOwnerName+"/repo-b")}
		}), condition: "repositories[1] id is not a positive integer"},
		{name: "owner id other than allowed owner id returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/example-repo"), effectiveRepository(repositoryIDB, otherOwnerID, allowedOwnerName+"/repo-b")}
		}), condition: "repositories[1] owner.id does not match allowed owner id"},
		{name: "missing permissions returns ErrVerificationFailure", body: modified(func(r map[string]any) { delete(r, "permissions") }), condition: "permissions are unreadable"},
		{name: "contents write returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["permissions"] = map[string]any{"contents": "write", "metadata": "read"}
		}), condition: "permissions do not equal allowed permissions except implied permissions"},
		{name: "missing contents returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["permissions"] = map[string]any{"metadata": "read"}
		}), condition: "permissions do not equal allowed permissions except implied permissions"},
		{name: "metadata write returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["permissions"] = map[string]any{"contents": "read", "metadata": "write"}
		}), condition: "permissions do not equal allowed permissions except implied permissions"},
		{name: "additional permission entry returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["permissions"] = map[string]any{"contents": "read", "metadata": "read", "issues": "read"}
		}), condition: "permissions do not equal allowed permissions except implied permissions"},
		{name: "missing repository_selection returns ErrVerificationFailure", body: modified(func(r map[string]any) { delete(r, "repository_selection") }), condition: "repository_selection is unreadable"},
		{name: "repository_selection all returns ErrVerificationFailure", body: modified(func(r map[string]any) { r["repository_selection"] = "all" }), condition: "repository_selection is not selected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := verifyResult(t, issue(t, tt.body(t)))
			if !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("Verify error = %v, want %v", err, ErrVerificationFailure)
			}
			if !reflect.ValueOf(got).IsZero() {
				t.Errorf("Verify returned non-zero InstallationToken with error")
			}
			message := err.Error()
			if !strings.Contains(message, tt.condition) {
				t.Errorf("error message %q does not contain condition %q", message, tt.condition)
			}
			if want := `github request id "` + requestID + `"`; !strings.Contains(message, want) {
				t.Errorf("error message %q does not contain %q", message, want)
			}
			hidden := []string{tokenMarker, allowedOwnerName, "repo-b", "repo-c", "example-repo", strconv.Itoa(repositoryIDA), strconv.Itoa(repositoryIDB), strconv.Itoa(allowedOwnerID), strconv.Itoa(otherOwnerID)}
			for _, value := range hidden {
				if strings.Contains(strings.ToLower(message), value) {
					t.Errorf("error message %q contains %q", message, value)
				}
			}
		})
	}
}

// modified は、記録した応答の形を modify で書き換えた本文を返す。
func modified(modify func(response map[string]any)) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		response := recordedIssuance()
		modify(response)
		return marshal(t, response)
	}
}
