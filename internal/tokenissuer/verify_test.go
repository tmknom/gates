package tokenissuer

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/permission"
)

func TestVerify_ReturnsReturnableIssuanceResult(t *testing.T) {
	result := issue(t, marshal(t, recordedIssuance()))

	got, err := verifyResult(t, result, contentsRead)
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
			if _, err := verifyResult(t, issue(t, marshal(t, response)), contentsRead); err != nil {
				t.Errorf("Verify returned error: %v", err)
			}
		})
	}
}

// grantedPermissions は、複数の権限と読み取り以外の水準を含む発行許可の Permissions である。
var grantedPermissions = map[string]string{"contents": "write", "issues": "read"}

//trace:checks AC-ISSUANCE-010
func TestVerify_AcceptsGrantedPermissionsWithImpliedPermissions(t *testing.T) {
	response := recordedIssuance()
	response["permissions"] = map[string]any{"contents": "write", "issues": "read", "metadata": "read"}

	got, err := verifyResult(t, issue(t, marshal(t, response)), grantedPermissions)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if want := mustNewPermissions(t, map[string]string{"contents": "write", "issues": "read", "metadata": "read"}); !got.EffectivePermissions().Equal(want) {
		t.Errorf("EffectivePermissions() is not equal to the permissions of the issuance result")
	}
}

// GitHub App に付与されていない権限を GitHub が除くか水準を下げて発行しても、トークンを返さないことを確かめる。
//
//trace:checks AC-AUTHZ-008
func TestVerify_RejectsPermissionsDifferentFromTokenGrant(t *testing.T) {
	tests := []struct {
		name        string
		permissions map[string]any
	}{
		{name: "issuance result without a granted permission returns ErrVerificationFailure", permissions: map[string]any{"contents": "write", "metadata": "read"}},
		{name: "issuance result with a lowered level returns ErrVerificationFailure", permissions: map[string]any{"contents": "read", "issues": "read", "metadata": "read"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := recordedIssuance()
			response["permissions"] = tt.permissions
			_, err := verifyResult(t, issue(t, marshal(t, response)), grantedPermissions)
			if !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("Verify error = %v, want %v", err, ErrVerificationFailure)
			}
			if want := "permissions do not equal token grant permissions except implied permissions"; !strings.Contains(err.Error(), want) {
				t.Errorf("error message %q does not contain condition %q", err.Error(), want)
			}
		})
	}
}

const otherOwnerID = 9876543

//trace:checks AC-AUTHZ-003 AC-ISSUANCE-003 AC-ISSUANCE-004
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
		}), condition: "permissions do not equal token grant permissions except implied permissions"},
		{name: "missing contents returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["permissions"] = map[string]any{"metadata": "read"}
		}), condition: "permissions do not equal token grant permissions except implied permissions"},
		{name: "metadata write returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["permissions"] = map[string]any{"contents": "read", "metadata": "write"}
		}), condition: "permissions do not equal token grant permissions except implied permissions"},
		{name: "additional permission entry returns ErrVerificationFailure", body: modified(func(r map[string]any) {
			r["permissions"] = map[string]any{"contents": "read", "metadata": "read", "issues": "read"}
		}), condition: "permissions do not equal token grant permissions except implied permissions"},
		{name: "missing repository_selection returns ErrVerificationFailure", body: modified(func(r map[string]any) { delete(r, "repository_selection") }), condition: "repository_selection is unreadable"},
		{name: "repository_selection all returns ErrVerificationFailure", body: modified(func(r map[string]any) { r["repository_selection"] = "all" }), condition: "repository_selection is not selected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := verifyResult(t, issue(t, tt.body(t)), contentsRead)
			if !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("Verify error = %v, want %v", err, ErrVerificationFailure)
			}
			if !reflect.ValueOf(got).IsZero() {
				t.Errorf("Verify returned non-zero ReturnableIssuanceResult with error")
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

//trace:checks AC-AUTHZ-012
func TestVerify_AcceptsProtectedRepositoryMatchingBothNameAndID(t *testing.T) {
	protectedRepositories := protectedRepositoriesOf(t, strconv.Itoa(repositoryIDB), "repo-b")
	if _, err := verify(issue(t, marshal(t, recordedIssuance())), tokenGrant(t, contentsRead), allowedOwner(t), protectedRepositories, now); err != nil {
		t.Errorf("Verify returned error: %v", err)
	}
}

// 名前と ID の対応の区別と文言は protected が検証する。ここでは、各実効対象の ID と完全名を渡すことと、同じ実効対象の既存の照合の後に判定することを確かめる
//
//trace:checks AC-AUTHZ-012 AC-ISSUANCE-004
func TestVerify_RejectsProtectedRepositoryMismatch(t *testing.T) {
	// recreated は、repo-b を削除して作り直す前の Repository ID で保護した一覧である
	const recreatedID = 9990001
	recreated := protectedRepositoriesOf(t, strconv.Itoa(recreatedID), "repo-b")
	tests := []struct {
		name      string
		body      func(t *testing.T) string
		condition string
	}{
		{
			name:      "effective repository mismatching a protected repository returns ErrVerificationFailure",
			body:      modified(func(map[string]any) {}),
			condition: "repositories[1]: protected repositories[",
		},
		{
			name: "effective repository failing the owner id and the protected repository returns the owner id condition",
			body: modified(func(r map[string]any) {
				r["repositories"] = []any{effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/example-repo"), effectiveRepository(repositoryIDB, otherOwnerID, allowedOwnerName+"/repo-b")}
			}),
			condition: "repositories[1] owner.id does not match allowed owner id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verify(issue(t, tt.body(t)), tokenGrant(t, contentsRead), allowedOwner(t), recreated, now)
			if !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("Verify error = %v, want %v", err, ErrVerificationFailure)
			}
			if !strings.Contains(err.Error(), tt.condition) {
				t.Errorf("error message %q does not contain condition %q", err.Error(), tt.condition)
			}
		})
	}
}

// 実効対象は要求と逆の順で、一方は大文字小文字だけが異なる。照合済みの値は、ポリシー読み取り許可の対象の順と表記で、対応する Repository ID を持つ。
//
//trace:checks AC-ISSUANCE-011
func TestVerifyPolicyReadToken_ReturnsVerifiedPolicyReadToken(t *testing.T) {
	result := issuePolicyRead(t, marshal(t, recordedPolicyReadIssuance()))

	got, err := verifyPolicyReadToken(result, policyReadGrant(t), allowedOwner(t), noProtectedRepositories(t), now)
	if err != nil {
		t.Fatalf("verifyPolicyReadToken returned error: %v", err)
	}
	if token, _ := result.PolicyReadToken(); got.token != token {
		t.Error("token is not the policy read token of the issuance result")
	}
	var targets []string
	for _, target := range got.targets {
		targets = append(targets, target.fullName.String()+" "+strconv.FormatInt(target.repositoryID.Int64(), 10))
	}
	want := []string{allowedOwnerName + "/repo-b " + strconv.Itoa(repositoryIDB), allowedOwnerName + "/Example-Repo " + strconv.Itoa(repositoryIDA)}
	if !reflect.DeepEqual(targets, want) {
		t.Errorf("targets = %v, want %v", targets, want)
	}
}

func TestVerifyPolicyReadToken_AcceptsSingleFileReadWithoutImpliedPermissions(t *testing.T) {
	response := recordedPolicyReadIssuance()
	response["permissions"] = map[string]any{"single_file": "read"}

	if _, err := verifyPolicyReadToken(issuePolicyRead(t, marshal(t, response)), policyReadGrant(t), allowedOwner(t), noProtectedRepositories(t), now); err != nil {
		t.Errorf("verifyPolicyReadToken returned error: %v", err)
	}
}

// 照合の条件は利用者に返すトークンと同じコードで確かめるため、ここでは各条件を代表する一つずつと、ポリシー読み取りトークンだけの条件を確かめる。
//
//trace:checks AC-ISSUANCE-011
func TestVerifyPolicyReadToken_Rejects(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(r map[string]any)
		condition string
	}{
		{name: "empty token returns ErrVerificationFailure", modify: func(r map[string]any) { r["token"] = "" }, condition: "token is unreadable"},
		{name: "expires_at equal to now returns ErrVerificationFailure", modify: func(r map[string]any) { r["expires_at"] = now.Format(time.RFC3339) }, condition: "expires_at is not after now"},
		{name: "missing repository returns ErrVerificationFailure", modify: func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDB, allowedOwnerID, allowedOwnerName+"/repo-b")}
		}, condition: "repositories count does not match policy read grant"},
		{name: "additional repository returns ErrVerificationFailure", modify: func(r map[string]any) {
			r["repositories"] = append(r["repositories"].([]any), effectiveRepository(1234567892, allowedOwnerID, allowedOwnerName+"/repo-c"))
		}, condition: "repositories count does not match policy read grant"},
		{name: "repository outside policy read grant returns ErrVerificationFailure", modify: func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/example-repo"), effectiveRepository(repositoryIDB, allowedOwnerID, allowedOwnerName+"/repo-c")}
		}, condition: "repositories[1] full_name does not match policy read grant"},
		{name: "owner id other than allowed owner id returns ErrVerificationFailure", modify: func(r map[string]any) {
			r["repositories"] = []any{effectiveRepository(repositoryIDA, allowedOwnerID, allowedOwnerName+"/example-repo"), effectiveRepository(repositoryIDB, otherOwnerID, allowedOwnerName+"/repo-b")}
		}, condition: "repositories[1] owner.id does not match allowed owner id"},
		{name: "additional contents read returns ErrVerificationFailure", modify: func(r map[string]any) {
			r["permissions"] = map[string]any{"single_file": "read", "contents": "read", "metadata": "read"}
		}, condition: "permissions do not equal policy read grant permissions except implied permissions"},
		{name: "single_file write returns ErrVerificationFailure", modify: func(r map[string]any) {
			r["permissions"] = map[string]any{"single_file": "write", "metadata": "read"}
		}, condition: "permissions do not equal policy read grant permissions except implied permissions"},
		{name: "missing single_file returns ErrVerificationFailure", modify: func(r map[string]any) {
			r["permissions"] = map[string]any{"metadata": "read"}
		}, condition: "permissions do not equal policy read grant permissions except implied permissions"},
		{name: "repository_selection all returns ErrVerificationFailure", modify: func(r map[string]any) { r["repository_selection"] = "all" }, condition: "repository_selection is not selected"},
		{name: "missing single_file_paths returns ErrVerificationFailure", modify: func(r map[string]any) { delete(r, "single_file_paths") }, condition: "single_file_paths are unreadable"},
		{name: "empty single_file_paths returns ErrVerificationFailure", modify: func(r map[string]any) { r["single_file_paths"] = []any{} }, condition: "single_file_paths are not only the repository policy path"},
		{name: "other path returns ErrVerificationFailure", modify: func(r map[string]any) { r["single_file_paths"] = []any{"README.md"} }, condition: "single_file_paths are not only the repository policy path"},
		{name: "path in other case returns ErrVerificationFailure", modify: func(r map[string]any) { r["single_file_paths"] = []any{".github/GATES-policy.yml"} }, condition: "single_file_paths are not only the repository policy path"},
		{name: "repository policy path with another path returns ErrVerificationFailure", modify: func(r map[string]any) {
			r["single_file_paths"] = []any{".github/gates-policy.yml", "README.md"}
		}, condition: "single_file_paths are not only the repository policy path"},
		{name: "duplicate repository policy path returns ErrVerificationFailure", modify: func(r map[string]any) {
			r["single_file_paths"] = []any{".github/gates-policy.yml", ".github/gates-policy.yml"}
		}, condition: "single_file_paths are not only the repository policy path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := recordedPolicyReadIssuance()
			tt.modify(response)

			got, err := verifyPolicyReadToken(issuePolicyRead(t, marshal(t, response)), policyReadGrant(t), allowedOwner(t), noProtectedRepositories(t), now)

			if !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("verifyPolicyReadToken error = %v, want %v", err, ErrVerificationFailure)
			}
			if !reflect.ValueOf(got).IsZero() {
				t.Error("verifyPolicyReadToken returned non-zero verifiedPolicyReadToken with error")
			}
			message := err.Error()
			if !strings.Contains(message, tt.condition) {
				t.Errorf("error message %q does not contain condition %q", message, tt.condition)
			}
			if want := `github request id "` + requestID + `"`; !strings.Contains(message, want) {
				t.Errorf("error message %q does not contain %q", message, want)
			}
			hidden := []string{tokenMarker, allowedOwnerName, "repo-b", "repo-c", "example-repo", "gates-policy", strconv.Itoa(repositoryIDA), strconv.Itoa(repositoryIDB), strconv.Itoa(allowedOwnerID), strconv.Itoa(otherOwnerID)}
			for _, value := range hidden {
				if strings.Contains(strings.ToLower(message), value) {
					t.Errorf("error message %q contains %q", message, value)
				}
			}
		})
	}
}

// ポリシー読み取りトークンの照合でも、すべての保護対象との組で名前と ID の対応を確かめる。
//
//trace:checks AC-AUTHZ-012 AC-ISSUANCE-011
func TestVerifyPolicyReadToken_RejectsProtectedRepositoryMismatch(t *testing.T) {
	recreated := protectedRepositoriesOf(t, "9990001", "repo-b")

	_, err := verifyPolicyReadToken(issuePolicyRead(t, marshal(t, recordedPolicyReadIssuance())), policyReadGrant(t), allowedOwner(t), recreated, now)

	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("verifyPolicyReadToken error = %v, want %v", err, ErrVerificationFailure)
	}
	if want := "repositories[1]: protected repositories["; !strings.Contains(err.Error(), want) {
		t.Errorf("error message %q does not contain condition %q", err.Error(), want)
	}
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

// modified は、記録した応答の形を modify で書き換えた本文を返す。
func modified(modify func(response map[string]any)) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		response := recordedIssuance()
		modify(response)
		return marshal(t, response)
	}
}
