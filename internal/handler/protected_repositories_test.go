package handler

import (
	"encoding/json/v2"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/caller"
)

// 保護対象の規則は protected、Authorization、Token Issuer が確かめる。
// ここでは、Handler が保持する一覧を事前認可と Grant に渡すことと、照合の一覧を持つ Token Issuer と組み合わせた発行処理の結果が、応答と監査記録に写ることを確かめる。

// allowedCallerName は、記録した発行応答の example-repo（Repository ID 1234567890）で、callerOf の Caller のリポジトリである。
const allowedCallerName = "example-repo"

// protectingRepoB は、記録した発行応答の repo-b（Repository ID 1234567891）を保護し、example-repo の Caller に contents:write までを許す一覧である。
const protectingRepoB = `repositories:
  - id: "1234567891"
    name: repo-b
    callers:
      - repository_id: "1234567890"
        repository_name: example-repo
        permissions:
          contents: write
`

// 許可 Caller の権限の上限より低い水準の要求で、発行 API に要求した水準を送り、要求した水準の実効権限を返すことを確かめる。
//
//trace:checks AC-AUTHZ-011
func TestHandle_AllowedCallerIsIssuedAtRequestedLevelBelowPermissionLimit(t *testing.T) {
	protectedRepositories := mustParseProtectedRepositories(t, protectingRepoB)
	tokenIssuer, issuanceBody := newTokenIssuerRecordingIssuance(t, protectedRepositories, recordedIssuance(), http.StatusNoContent)

	inv := handleWith(t, mustNewIssuablePermissions(t, "contents:write"), protectedRepositories, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	if inv.response["ok"] != true {
		t.Fatalf("response = %v, want the success response", inv.response)
	}
	var sent struct {
		Permissions map[string]string `json:"permissions"`
	}
	if err := json.Unmarshal(*issuanceBody.Load(), &sent); err != nil {
		t.Fatalf("issuance request body is not a JSON object: %v", err)
	}
	if want := map[string]string{"contents": "read"}; !reflect.DeepEqual(sent.Permissions, want) {
		t.Errorf("issuance request permissions = %v, want %v", sent.Permissions, want)
	}
	if want := map[string]any{"contents": "read", "metadata": "read"}; !reflect.DeepEqual(inv.response["permissions"], want) {
		t.Errorf("response permissions = %v, want %v", inv.response["permissions"], want)
	}
}

// Caller のリポジトリ名だけが許可 Caller と異なる要求を、Token Issuer を呼ばずに拒否することを確かめる。
// Token Issuer は KMS Signer と GitHub Client を使う唯一の部品のため、呼ばれないことで両方が呼ばれないことを確かめる。
//
//trace:checks AC-AUTHZ-011
func TestHandle_CallerNotMatchingAllowedCallerNameIsDeniedWithoutTokenIssuer(t *testing.T) {
	const callerNameMarker = "caller-name-marker"
	verified, err := caller.NewCaller(allowedOwnerID, 1234567890, callerNameMarker, 4004, 1, caller.WorkflowTriggerAttributes{})
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	tokenIssuer := &tokenIssuerStub{}

	inv := handleWith(t, mustNewIssuablePermissions(t, "contents:read"), mustParseProtectedRepositories(t, protectingRepoB), &verifierStub{caller: verified}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "authorization_denied")
	if tokenIssuer.calls != 0 {
		t.Errorf("Token Issuer calls = %d, want 0", tokenIssuer.calls)
	}
	reason, _ := record["reason"].(string)
	// 許可 Caller の改名を示す理由が、監査記録の reason で読めることを確かめる（Lambda Design の「Audit Record」）
	if want := "matches the caller repository id but not the name"; !strings.Contains(reason, want) {
		t.Errorf("reason = %q, want it to contain %q", reason, want)
	}
	assertHidesProtectedRepositories(t, inv, reason, callerNameMarker, allowedCallerName)
}

// 一覧を直さずに保護対象を改名した場合と、削除して同じ名前で作り直した場合に、トークンを返さず、失効の結果を監査記録に残すことを確かめる。
//
//trace:checks AC-AUTHZ-012 AC-ISSUANCE-004
func TestHandle_ProtectedRepositoryMismatchFailsVerificationAndRevokes(t *testing.T) {
	tests := []struct {
		name             string
		document         string
		revocationStatus int
		wantReason       string
		wantRevocation   string
	}{
		{
			// 改名前の名前 old-name で保護したまま、新しい名前 repo-b で要求する。名前が一致しないため、事前認可は保護対象として判定しない
			name: "renamed protected repository with successful revocation",
			document: `repositories:
  - id: "1234567891"
    name: old-name
    callers: []
`,
			revocationStatus: http.StatusNoContent,
			wantReason:       "repositories[1]: protected repositories[0]",
			wantRevocation:   "succeeded",
		},
		{
			// 作り直す前の Repository ID 9990001 で保護したまま、許可 Caller から同じ名前 repo-b で要求する
			name: "recreated protected repository with failed revocation",
			document: `repositories:
  - id: "9990001"
    name: repo-b
    callers:
      - repository_id: "1234567890"
        repository_name: example-repo
        permissions:
          contents: read
`,
			revocationStatus: http.StatusInternalServerError,
			wantReason:       "repositories[1]: protected repositories[0]",
			wantRevocation:   "failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protectedRepositories := mustParseProtectedRepositories(t, tt.document)
			tokenIssuer, _ := newTokenIssuerRecordingIssuance(t, protectedRepositories, recordedIssuance(), tt.revocationStatus)

			inv := handleWith(t, mustNewIssuablePermissions(t, "contents:read"), protectedRepositories, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

			record := inv.assertFailed(t, "issuance_verification_failed")
			if record["revocation"] != tt.wantRevocation {
				t.Errorf("revocation = %v, want %q", record["revocation"], tt.wantRevocation)
			}
			reason, _ := record["reason"].(string)
			if !strings.Contains(reason, tt.wantReason) {
				t.Errorf("reason = %q, want it to contain %q", reason, tt.wantReason)
			}
			assertHidesProtectedRepositories(t, inv, reason, "old-name", "9990001", "1234567891", "repo-b")
		})
	}
}

// assertHidesProtectedRepositories は、応答と監査記録の reason に、保護対象と許可 Caller の ID と名前、権限の上限の水準が入らないことを確かめる。
// 要求の対象は監査記録の request に入るため、ログ全体ではなく reason を調べる。
func assertHidesProtectedRepositories(t *testing.T, inv invocation, reason string, values ...string) {
	t.Helper()
	response := string(marshal(t, inv.response))
	for _, value := range append(values, "1234567890", "write") {
		if strings.Contains(response, value) {
			t.Errorf("response %s contains %q", response, value)
		}
		if strings.Contains(reason, value) {
			t.Errorf("reason %q contains %q", reason, value)
		}
	}
}
