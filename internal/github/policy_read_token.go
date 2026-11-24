package github

import (
	"github.com/tmknom/gates/internal/credential"
	"github.com/tmknom/gates/internal/nonzero"
)

// PolicyReadToken は、ポリシー読み取りトークンの発行応答から読み取ったポリシー読み取りトークンである。
// GitHub の上では Installation Access Token と同じ形だが、値を取り出せるのが GitHub Client だけのため、別の型にして公開の取り出し口を持たせない。
type PolicyReadToken struct {
	credential.Redacted
	// credential.Redacted の規則に従い、ポインタで持つ
	value *string
}

// IsZero は、発行応答から読み取っていないゼロ値かを返す。
func (t PolicyReadToken) IsZero() bool {
	return t.value == nil
}

// expose は、GitHub API の認証に使う値を返す。
func (t PolicyReadToken) expose() string {
	// ゼロ値は発行応答から読み取ったトークンではなく、送る値がない
	nonzero.Require(t)
	return *t.value
}
