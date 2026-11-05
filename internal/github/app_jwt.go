package github

import (
	"github.com/tmknom/gates/internal/credential"
	"github.com/tmknom/gates/internal/nonzero"
)

// AppJWT は、KMS Signer が作り、GitHub Client が GitHub API の認証に使う GitHub App JWT である。
// 値を取り出す口を非公開にし、GitHub Client の外で値を読めないことを型で守る。
// ゼロ値は KMS Signer が error とともに返す値である。
type AppJWT struct {
	credential.Redacted
	// credential.Redacted の規則に従い、ポインタで持つ
	value *string
}

// NewAppJWT は、JWS Compact Serialization の GitHub App JWT を AppJWT にする。呼ぶのは KMS Signer だけである。
// 形を検証しないのは、形を保証するのが組み立てる KMS Signer で、受け付けるかを判定するのが GitHub だからである。
func NewAppJWT(compact string) AppJWT {
	return AppJWT{value: &compact}
}

// IsZero は、NewAppJWT を経ていないゼロ値かを返す。
func (j AppJWT) IsZero() bool {
	return j.value == nil
}

// expose は、GitHub API の認証に使う値を返す。
func (j AppJWT) expose() string {
	// ゼロ値は KMS Signer が error とともに返す値で、認証に送る値がない
	nonzero.Require(j)
	return *j.value
}
