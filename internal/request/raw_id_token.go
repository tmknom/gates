package request

import (
	"fmt"
	"log/slog"
)

const redacted = "[REDACTED]"

// RawIDToken は、Payload の id_token から取り出した、認証前の GATES 用 OIDC JWT である。
// 資格情報をログとエラー本文に出さないため、fmt、slog、JSON のどの出力でも値を伏せる。
type RawIDToken struct {
	// fmt はほかの struct の非公開フィールドに置かれた値の Format を呼べず、フィールドをそのまま出す。
	// ポインタで持ち、そのときにもアドレスしか出ないようにする
	value *string
}

// Expose は、JWT の値を返す。認証のために OIDC Verifier だけが呼ぶ。
// ゼロ値は Validate が error とともに返す値で、Expose を呼ぶと panic する。
func (t RawIDToken) Expose() string {
	return *t.value
}

// Format は、どの書式指定でも伏せた表記を出す。Formatter は String と GoString より優先されるため、これだけで足りる。
func (t RawIDToken) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(redacted)) // fmt.State の Write は fmt の内部バッファに追記するだけで、error を返さない
}

// LogValue は、slog の attribute として伏せた表記を返す。
func (t RawIDToken) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON は、伏せた表記を JSON の文字列として返す。encoding/json と encoding/json/v2 のどちらもこのメソッドを使う。
func (t RawIDToken) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redacted + `"`), nil
}
