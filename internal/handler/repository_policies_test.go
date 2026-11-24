package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/revocation"
	"github.com/tmknom/gates/internal/tokenissuer"
)

// 事前認可が件数の上限で拒否し、KMS と GitHub API を呼ぶ Token Issuer に進まないことを確かめる。
//
//trace:checks AC-AUTHZ-017 AC-ISSUANCE-011
func TestHandle_RepositoryCountOverLimitUnderEnforcementDoesNotCallTokenIssuer(t *testing.T) {
	tokenIssuer := &tokenIssuerStub{}

	inv := handleEnforced(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, payloadOfRepositories(t, 11))

	record := inv.assertFailed(t, "authorization_denied")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller")...)
	if reason, _ := record["reason"].(string); !strings.Contains(reason, "repositories count exceeds the limit under repository policy enforcement") {
		t.Errorf("reason = %q, want the repository count limit", reason)
	}
	if tokenIssuer.calls != 0 {
		t.Errorf("Token Issuer calls = %d, want 0", tokenIssuer.calls)
	}
}

func TestHandle_RepositoryCountOverLimitWithoutEnforcementCallsTokenIssuer(t *testing.T) {
	tokenIssuer := &tokenIssuerStub{err: github.ErrTransientFailure}

	inv := handle(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, payloadOfRepositories(t, 11))

	inv.assertFailed(t, "github_api_unavailable")
	if tokenIssuer.calls != 1 {
		t.Errorf("Token Issuer calls = %d, want 1", tokenIssuer.calls)
	}
}

//trace:checks AC-AUTHZ-014 AC-ISSUANCE-011
func TestHandle_IssuesTokenAllowedByRepositoryPolicies(t *testing.T) {
	tokenIssuer, _ := newEnforcedTokenIssuer(t, noProtectedRepositories(t), enforcedGitHub{
		policyReadIssuance: recordedPolicyReadIssuance(),
		issuance:           recordedIssuance(),
		policies:           map[string]http.HandlerFunc{"repo-b": repositoryPolicyFile(t, allowingRepositoryPolicy()), "example-repo": repositoryPolicyFile(t, allowingRepositoryPolicy())},
	})

	inv := handleEnforced(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	if inv.response["ok"] != true || inv.response["token"] != tokenMarker {
		t.Fatalf("response = %v, want the success response with the token to return", inv.response)
	}
	record := inv.auditRecord(t)
	assertKeys(t, "audit record", record, auditKeys("caller", "request", "installation", "policy_read_revocation")...)
	if record["policy_read_revocation"] != "succeeded" {
		t.Errorf("policy_read_revocation = %v, want succeeded", record["policy_read_revocation"])
	}
}

// リポジトリポリシーが許す対象を混ぜても全体を拒否し、利用者に返すトークンの発行 API を呼ばないことを確かめる。
//
//trace:checks AC-AUTHZ-014 AC-ISSUANCE-011
func TestHandle_RepositoryPolicyDenialIssuesNoToken(t *testing.T) {
	tokenIssuer, issuanceBody := newEnforcedTokenIssuer(t, noProtectedRepositories(t), enforcedGitHub{
		policyReadIssuance: recordedPolicyReadIssuance(),
		issuance:           recordedIssuance(),
		policies:           map[string]http.HandlerFunc{"repo-b": repositoryPolicyFile(t, allowingRepositoryPolicy()), "example-repo": repositoryPolicyFile(t, denyingRepositoryPolicy())},
	})

	inv := handleEnforced(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "authorization_denied")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller", "installation", "policy_read_revocation")...)
	// 拒否の理由の区別と文言は repositorypolicy が検証する。ここでは拒否した対象の位置とリポジトリポリシーによる拒否であることを確かめる
	if reason, want := fmt.Sprint(record["reason"]), "authorization denied: repositories[1]: repository policy "; !strings.HasPrefix(reason, want) {
		t.Errorf("reason = %q, want it to start with %q", reason, want)
	}
	if record["policy_read_revocation"] != "succeeded" {
		t.Errorf("policy_read_revocation = %v, want succeeded", record["policy_read_revocation"])
	}
	if body := issuanceBody.Load(); body != nil {
		t.Errorf("the token to return was issued with %s, want no issuance", *body)
	}
}

//trace:checks AC-AUTHZ-015
func TestHandle_MissingRepositoryPolicyIssuesNoToken(t *testing.T) {
	tokenIssuer, issuanceBody := newEnforcedTokenIssuer(t, noProtectedRepositories(t), enforcedGitHub{
		policyReadIssuance: recordedPolicyReadIssuance(),
		issuance:           recordedIssuance(),
		policies:           map[string]http.HandlerFunc{"repo-b": respond(t, http.StatusNotFound, map[string]any{"message": "Not Found"}), "example-repo": repositoryPolicyFile(t, allowingRepositoryPolicy())},
	})

	inv := handleEnforced(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "authorization_denied")
	if reason, want := fmt.Sprint(record["reason"]), "authorization denied: repositories[0]: repository policy "; !strings.HasPrefix(reason, want) {
		t.Errorf("reason = %q, want it to start with %q", reason, want)
	}
	if body := issuanceBody.Load(); body != nil {
		t.Errorf("the token to return was issued with %s, want no issuance", *body)
	}
}

// 保護対象の一覧が許す要求も、リポジトリポリシーが許さなければ拒否することを確かめる。
//
//trace:checks AC-AUTHZ-016
func TestHandle_ProtectedRepositoryAllowedOnlyByProtectedRepositoriesIsDenied(t *testing.T) {
	protectedRepositories := mustParseProtectedRepositories(t, "repositories:\n  - id: \"1234567891\"\n    name: repo-b\n    callers:\n      - repository_id: \"1234567890\"\n        repository_name: example-repo\n        permissions:\n          contents: read\n")
	tokenIssuer, issuanceBody := newEnforcedTokenIssuer(t, protectedRepositories, enforcedGitHub{
		policyReadIssuance: recordedPolicyReadIssuance(),
		issuance:           recordedIssuance(),
		policies:           map[string]http.HandlerFunc{"repo-b": repositoryPolicyFile(t, denyingRepositoryPolicy()), "example-repo": repositoryPolicyFile(t, allowingRepositoryPolicy())},
	})

	inv := handleUnder(t, authz.RepositoryPolicyEnforced, mustNewIssuablePermissions(t, "contents:read"), protectedRepositories, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "authorization_denied")
	if reason, want := fmt.Sprint(record["reason"]), "authorization denied: repositories[0]: repository policy "; !strings.HasPrefix(reason, want) {
		t.Errorf("reason = %q, want it to start with %q", reason, want)
	}
	if body := issuanceBody.Load(); body != nil {
		t.Errorf("the token to return was issued with %s, want no issuance", *body)
	}
}

//trace:checks AC-QUALITY-001 AC-ISSUANCE-011
func TestHandle_RepositoryPolicyFetchFailureWritesPolicyReadRevocation(t *testing.T) {
	err := fmt.Errorf("fetch repository policy: repositories[0]: %w", github.ErrTransientFailure)
	tokenIssuer := &tokenIssuerStub{err: err, policyReadRevocation: revocation.Failed}

	inv := handleEnforced(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "github_api_unavailable")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller", "policy_read_revocation")...)
	if record["policy_read_revocation"] != "failed" {
		t.Errorf("policy_read_revocation = %v, want failed", record["policy_read_revocation"])
	}
}

// panic を回復した記録は Token Issuer の結果を受け取っていないため、ポリシー読み取りトークンの失効の結果を持たない。
func TestHandle_RecoveredPanicDoesNotWritePolicyReadRevocation(t *testing.T) {
	// 失効の結果のない照合失敗は、監査記録の結果を作るときに panic する
	err := fmt.Errorf("issue: %w", tokenissuer.ErrVerificationFailure)
	tokenIssuer := &tokenIssuerStub{err: err, policyReadRevocation: revocation.Succeeded}

	inv := handleEnforced(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "internal_error")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller")...)
	if record["reason"] != reasonPanicRecovered {
		t.Errorf("reason = %v, want %q", record["reason"], reasonPanicRecovered)
	}
}

//trace:checks AC-ISSUANCE-011
func TestHandle_PolicyReadTokenVerificationFailureWritesBothRevocations(t *testing.T) {
	policyReadIssuance := recordedPolicyReadIssuance()
	policyReadIssuance["single_file_paths"] = []any{"README.md"}
	tokenIssuer, issuanceBody := newEnforcedTokenIssuer(t, noProtectedRepositories(t), enforcedGitHub{
		policyReadIssuance: policyReadIssuance,
		issuance:           recordedIssuance(),
	})

	inv := handleEnforced(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "issuance_verification_failed")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller", "installation", "revocation", "policy_read_revocation")...)
	if reason, _ := record["reason"].(string); !strings.HasPrefix(reason, "issue policy read token: ") {
		t.Errorf("reason = %q, want the stage of the policy read token", reason)
	}
	if record["revocation"] != "not_attempted" || record["policy_read_revocation"] != "succeeded" {
		t.Errorf("revocation = %v, policy_read_revocation = %v, want not_attempted and succeeded", record["revocation"], record["policy_read_revocation"])
	}
	if body := issuanceBody.Load(); body != nil {
		t.Errorf("the token to return was issued with %s, want no issuance", *body)
	}
}

//trace:checks AC-ISSUANCE-012
func TestHandle_RepositoryIDDifferentFromReadTargetFailsVerification(t *testing.T) {
	issuance := recordedIssuance()
	repositories, _ := issuance["repositories"].([]any)
	repositories[1] = map[string]any{"id": 1234567892, "full_name": allowedOwnerName + "/repo-b", "owner": map[string]any{"id": allowedOwnerID}}
	tokenIssuer, _ := newEnforcedTokenIssuer(t, noProtectedRepositories(t), enforcedGitHub{
		policyReadIssuance: recordedPolicyReadIssuance(),
		issuance:           issuance,
		policies:           map[string]http.HandlerFunc{"repo-b": repositoryPolicyFile(t, allowingRepositoryPolicy()), "example-repo": repositoryPolicyFile(t, allowingRepositoryPolicy())},
	})

	inv := handleEnforced(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "issuance_verification_failed")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller", "installation", "revocation", "policy_read_revocation")...)
	if reason, _ := record["reason"].(string); !strings.Contains(reason, "id does not match the repository whose repository policy was read") {
		t.Errorf("reason = %q, want the mismatch with the repository whose repository policy was read", reason)
	}
	if record["revocation"] != "succeeded" || record["policy_read_revocation"] != "succeeded" {
		t.Errorf("revocation = %v, policy_read_revocation = %v, want both succeeded", record["revocation"], record["policy_read_revocation"])
	}
}

// ポリシー読み取り許可の確定で拒否しても、Installation の解決には成功しているため、Installation を記録する。
func TestHandle_PolicyReadGrantRejectionKeepsResolvedInstallation(t *testing.T) {
	tokenIssuer := &tokenIssuerStub{installation: installation(t, allowedOwnerID, true), enforced: true}

	inv := handleEnforced(t, &verifierStub{caller: verifiedCaller(t)}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "installation_unavailable")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller", "installation")...)
}

// handleEnforced は、リポジトリポリシーを適用し、発行上限を `contents:read` にし、保護対象のない一覧を持つ Handler で handleUnder を呼ぶ。
func handleEnforced(t *testing.T, verifier oidcVerifier, tokenIssuer installationAccessTokenIssuer, payload []byte) invocation {
	t.Helper()
	return handleUnder(t, authz.RepositoryPolicyEnforced, mustNewIssuablePermissions(t, "contents:read"), noProtectedRepositories(t), verifier, tokenIssuer, payload)
}

// payloadOfRepositories は、許可オーナーの n 件の対象に contents:read を要求する Payload である。
func payloadOfRepositories(t *testing.T, n int) []byte {
	t.Helper()
	repositories := make([]string, n)
	for i := range repositories {
		repositories[i] = fmt.Sprintf("%s/repo-%d", allowedOwnerName, i)
	}
	return marshal(t, map[string]any{
		"version":      1,
		"id_token":     oidcJWTMarker,
		"repositories": repositories,
		"permissions":  map[string]string{"contents": "read"},
	})
}
