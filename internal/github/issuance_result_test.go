package github_test

import (
	"strings"
	"testing"
)

func TestIssuanceResult_EffectiveRepositoriesCannotBeModifiedByCaller(t *testing.T) {
	result := issueBody(t, marshal(t, recordedIssuance()))

	got, _ := result.EffectiveRepositories()
	got[0] = got[1]

	again, _ := result.EffectiveRepositories()
	if again[0].RepositoryID() != 1234567890 {
		t.Errorf("EffectiveRepositories()[0].RepositoryID() = %d after modifying the copy, want 1234567890", again[0].RepositoryID())
	}
}

func TestInstallationAccessToken_ExposeReturnsTokenAsIs(t *testing.T) {
	// GitHub Constraints が新形式のトークンの長さとして挙げる約 520 文字
	token := "ghs_" + strings.Repeat("A1b2_", 103) + "x"
	issuance := recordedIssuance()
	issuance["token"] = token

	got, ok := issueBody(t, marshal(t, issuance)).Token()

	if !ok {
		t.Fatal("Token() was not read")
	}
	if got.Expose() != token {
		t.Errorf("Token().Expose() = %d characters, want the %d characters of the response", len(got.Expose()), len(token))
	}
}
