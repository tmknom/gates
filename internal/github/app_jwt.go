package github

import (
	"fmt"
	"log/slog"
)

const redacted = "[REDACTED]"

// AppJWT は、KMS Signer が作り、GitHub Client が GitHub API の認証に使う GitHub App JWT である。
// 資格情報をログとエラー本文に出さないため、fmt、slog、JSON のどの出力でも値を伏せる。
// 値を取り出すメソッドを非公開にし、GitHub Client の外で値を読めないことを型で守る。
type AppJWT struct {
	// fmt はほかの struct の非公開フィールドに置かれた値の Format を呼べず、フィールドをそのまま出す。
	// ポインタで持ち、そのときにもアドレスしか出ないようにする
	value *string
}

// NewAppJWT は、JWS Compact Serialization の GitHub App JWT を AppJWT にする。呼ぶのは KMS Signer だけである。
// 形を検証しないのは、形を保証するのが組み立てる KMS Signer で、受け付けるかを判定するのが GitHub だからである。
func NewAppJWT(compact string) AppJWT {
	return AppJWT{value: &compact}
}

// expose は、GitHub API の認証に使う値を返す。
// ゼロ値は KMS Signer が error とともに返す値で、expose を呼ぶと panic する。
func (t AppJWT) expose() string {
	return *t.value
}

// Format は、どの書式指定でも伏せた表記を出す。Formatter は String と GoString より優先されるため、これだけで足りる。
func (t AppJWT) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(redacted)) // fmt.State の Write は fmt の内部バッファに追記するだけで、error を返さない
}

// LogValue は、slog の attribute として伏せた表記を返す。
func (t AppJWT) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON は、伏せた表記を JSON の文字列として返す。encoding/json と encoding/json/v2 のどちらもこのメソッドを使う。
func (t AppJWT) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redacted + `"`), nil
}
