package github

import (
	"github.com/tmknom/gates/internal/credential"
	"github.com/tmknom/gates/internal/nonzero"
)

// InstallationAccessToken は、発行応答から読み取った Installation Access Token である。
// 値を取り出せるコンポーネントは、Lambda Design の「Logical Components」の資格情報の表に従う。
type InstallationAccessToken struct {
	credential.Redacted
	// credential.Redacted の規則に従い、ポインタで持つ
	value *string
}

// IsZero は、発行応答から読み取っていないゼロ値かを返す。
func (t InstallationAccessToken) IsZero() bool {
	return t.value == nil
}

// Expose は、成功応答に入れる値を返す。成功応答を書くために Lambda Handler だけが呼ぶ。
// bearer の expose と分けるのは、bearer のメソッドを公開すると GitHub App JWT の値まで package の外から読めるためである。
func (t InstallationAccessToken) Expose() string {
	return t.expose()
}

// expose は、GitHub API の認証に使う値を返す。
func (t InstallationAccessToken) expose() string {
	// ゼロ値は発行応答から読み取ったトークンではなく、送る値も応答に入れる値もない
	nonzero.Require(t)
	return *t.value
}
