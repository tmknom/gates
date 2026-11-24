package github_test

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/protected"
	"github.com/tmknom/gates/internal/request"
)

const (
	recordedInstallationID = 12345678
	recordedAccountID      = 583231

	// tokenMarker は、トークンがエラー文言に漏れていないことを確かめる目印である。
	tokenMarker = "ghs_token-marker" //nolint:gosec // 資格情報ではなく、漏れを探すための目印である
)

const (
	appJWTValue      = "secret.app.jwt"
	allowedOwnerName = "allowed-owner"
)

// requestedRepositories は、発行許可の対象である。表記と並びが要求のまま発行 API に渡ることを確かめるため、大文字を含め、名前の昇順にしない。
var requestedRepositories = []string{allowedOwnerName + "/repo-b", allowedOwnerName + "/Example-Repo"}

// contentsRead は、調査記録の発行応答を得た要求の Permissions である。
var contentsRead = map[string]string{"contents": "read"}

// recordedInstallation は、調査記録の Installation の応答を、ケースごとに書き換えられる形で返す。
func recordedInstallation() map[string]any {
	return map[string]any{
		"id":        recordedInstallationID,
		"client_id": "Iv23liEXAMPLE0000000",
		"account": map[string]any{
			"login": "octocat",
			"id":    recordedAccountID,
			"type":  "User",
		},
		"repository_selection": "selected",
		"app_id":               123456,
		"target_id":            recordedAccountID,
		"target_type":          "User",
		"permissions":          map[string]any{"contents": "read", "metadata": "read"},
		"events":               []any{},
		"created_at":           "2026-10-28T04:32:35.000Z",
		"suspended_by":         nil,
		"suspended_at":         nil,
	}
}

// recordedIssuance は、調査記録の発行応答を、ケースごとに書き換えられる形で返す。リポジトリの表現は照合に使うキーと一部のキーだけにする。
func recordedIssuance() map[string]any {
	return map[string]any{
		"token":                tokenMarker,
		"expires_at":           "2026-11-03T12:18:48Z",
		"permissions":          map[string]any{"contents": "read", "metadata": "read"},
		"repository_selection": "selected",
		"repositories": []any{
			map[string]any{
				"id":        1234567890,
				"name":      "example-repo",
				"full_name": "octocat/example-repo",
				"private":   true,
				"owner":     map[string]any{"login": "octocat", "id": recordedAccountID, "type": "User"},
			},
			map[string]any{
				"id":        1234567891,
				"name":      "repo-b",
				"full_name": "octocat/repo-b",
				"private":   true,
				"owner":     map[string]any{"login": "octocat", "id": recordedAccountID, "type": "User"},
			},
		},
	}
}

// recordedPolicyReadIssuance は、ポリシー読み取りトークンの発行応答を、ケースごとに書き換えられる形で返す。
// 調査記録は Single file のパスを README.md にした発行応答で、パスの値だけをリポジトリポリシーのパスに置き換えている。
func recordedPolicyReadIssuance() map[string]any {
	body := recordedIssuance()
	body["permissions"] = map[string]any{"single_file": "read", "metadata": "read"}
	body["single_file"] = ".github/gates-policy.yml"
	body["has_multiple_single_files"] = false
	body["single_file_paths"] = []any{".github/gates-policy.yml"}
	return body
}

func allowedOwner(t *testing.T, ownerType authz.OwnerType) authz.AllowedOwner {
	t.Helper()
	owner, err := authz.NewAllowedOwner(recordedAccountID, allowedOwnerName, ownerType)
	if err != nil {
		t.Fatalf("NewAllowedOwner returned error: %v", err)
	}
	return owner
}

func newInstallation(t *testing.T, suspended bool) authz.Installation {
	t.Helper()
	installation, err := authz.NewInstallation(recordedInstallationID, recordedAccountID, suspended)
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	return installation
}

func resolve(t *testing.T, client *github.Client) (authz.Installation, error) {
	t.Helper()
	return client.ResolveInstallation(t.Context(), github.NewAppJWT(appJWTValue), allowedOwner(t, authz.OwnerTypeUser))
}

// tokenGrant は、TokenGrant を Grant でしか作れないため、grantInputs のそろえた値と Installation で Grant を通す。
func tokenGrant(t *testing.T, permissions map[string]string) authz.TokenGrant {
	t.Helper()
	inputs := newGrantInputs(t, permissions)
	grant, err := authz.Grant(inputs.caller, inputs.request, allowedOwner(t, authz.OwnerTypeUser), inputs.issuablePermissions, inputs.protectedRepositories, newInstallation(t, false))
	if err != nil {
		t.Fatalf("Grant returned error: %v", err)
	}
	return grant
}

// policyReadGrant は、PolicyReadGrant を GrantPolicyRead でしか作れないため、grantInputs のそろえた値と Installation で GrantPolicyRead を通す。
func policyReadGrant(t *testing.T) authz.PolicyReadGrant {
	t.Helper()
	inputs := newGrantInputs(t, contentsRead)
	grant, err := authz.GrantPolicyRead(inputs.caller, inputs.request, allowedOwner(t, authz.OwnerTypeUser), inputs.issuablePermissions, inputs.protectedRepositories, newInstallation(t, false))
	if err != nil {
		t.Fatalf("GrantPolicyRead returned error: %v", err)
	}
	return grant
}

// grantInputs は、発行許可とポリシー読み取り許可の確定に渡す Caller、形式検証を通した発行要求、発行上限、保護対象の一覧である。
type grantInputs struct {
	caller                caller.Caller
	request               request.TokenRequest
	issuablePermissions   permission.IssuablePermissions
	protectedRepositories protected.Repositories
}

// newGrantInputs は、requestedRepositories を対象とする要求の値をそろえる。
// 発行上限は、認可を判定し直しても通るよう、要求の Permissions と同じ組にする。
func newGrantInputs(t *testing.T, permissions map[string]string) grantInputs {
	t.Helper()
	c, err := caller.NewCaller(recordedAccountID, 1234567890, "example-repo", 4004, 1, caller.WorkflowTriggerAttributes{})
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
	// 保護対象のない一覧にする。保護対象の条件は Authorization と Token Issuer が確かめるため
	noProtectedRepositories, err := protected.Parse("repositories: []\n")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	return grantInputs{caller: c, request: req, issuablePermissions: issuablePermissions, protectedRepositories: noProtectedRepositories}
}

func issue(t *testing.T, client *github.Client) (github.IssuanceResult, error) {
	t.Helper()
	return client.Issue(t.Context(), github.NewAppJWT(appJWTValue), tokenGrant(t, contentsRead))
}

func issueBody(t *testing.T, body string) github.IssuanceResult {
	t.Helper()
	got, err := issue(t, newAPIStub(t, respondIssuance(body)).client(t))
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	return got
}

func issuePolicyReadToken(t *testing.T, client *github.Client) (github.IssuanceResult, error) {
	t.Helper()
	return client.IssuePolicyReadToken(t.Context(), github.NewAppJWT(appJWTValue), policyReadGrant(t))
}

func issuePolicyReadTokenBody(t *testing.T, body string) github.IssuanceResult {
	t.Helper()
	got, err := issuePolicyReadToken(t, newAPIStub(t, respondIssuance(body)).client(t))
	if err != nil {
		t.Fatalf("IssuePolicyReadToken returned error: %v", err)
	}
	return got
}

// issuedPolicyReadToken は、ポリシー読み取りトークンの発行応答から読み取ったポリシー読み取りトークンを返す。
func issuedPolicyReadToken(t *testing.T) github.PolicyReadToken {
	t.Helper()
	token, ok := issuePolicyReadTokenBody(t, marshal(t, recordedPolicyReadIssuance())).PolicyReadToken()
	if !ok {
		t.Fatal("PolicyReadToken() is not read from the recorded policy read issuance response")
	}
	return token
}

func marshal(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return string(b)
}

// padded は JSON の後ろに空白を足し、復号できる本文のまま大きさだけを size に揃える。
func padded(body string, size int) string {
	return body + strings.Repeat(" ", size-len(body))
}
