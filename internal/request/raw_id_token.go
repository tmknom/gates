package request

import "github.com/tmknom/gates/internal/credential"

// RawIDToken は、Payload の id_token から取り出した、認証前の GATES 用 OIDC JWT である。
type RawIDToken struct {
	credential.Redacted
	// credential.Redacted の規則に従い、ポインタで持つ
	value *string
}

// Expose は、JWT の値を返す。認証のために OIDC Verifier だけが呼ぶ。
// ゼロ値は Validate が error とともに返す値で、Expose を呼ぶと panic する。
func (t RawIDToken) Expose() string {
	return *t.value
}
