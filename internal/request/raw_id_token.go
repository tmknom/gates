package request

import (
	"github.com/tmknom/gates/internal/credential"
	"github.com/tmknom/gates/internal/nonzero"
)

// RawIDToken は、Payload の id_token から取り出した、認証前の GATES 用 OIDC JWT である。
type RawIDToken struct {
	credential.Redacted
	// credential.Redacted の規則に従い、ポインタで持つ
	value *string
}

// IsZero は、Validate を経ていないゼロ値かを返す。
func (t RawIDToken) IsZero() bool {
	return t.value == nil
}

// Expose は、JWT の値を返す。認証のために OIDC Verifier だけが呼ぶ。
func (t RawIDToken) Expose() string {
	// ゼロ値は Validate が error とともに返す値で、認証する値がない
	nonzero.Require(t)
	return *t.value
}
