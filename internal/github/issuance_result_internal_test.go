package github

import (
	"strings"
	"testing"
)

// トークンの値は GitHub Client の外から読めないため、値をそのまま読み取ることは package の中で確かめる。
func TestDecodeIssuanceResult_ReadsTokenAsIs(t *testing.T) {
	// GitHub Constraints が新形式のトークンの長さとして挙げる約 520 文字
	token := "ghs_" + strings.Repeat("A1b2_", 103) + "x"

	got, ok := decodeIssuanceResult([]byte(`{"token":"` + token + `"}`)).Token()

	if !ok {
		t.Fatal("Token() was not read")
	}
	if got.expose() != token {
		t.Errorf("Token() = %d characters, want the %d characters of the response", len(got.expose()), len(token))
	}
}
