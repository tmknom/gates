package github_test

import (
	"strings"
	"testing"
)

func TestInstallationAccessToken_ExposeReturnsTokenAsIs(t *testing.T) {
	// GitHub App Constraints が新形式のトークンの長さとして挙げる約 520 文字
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
