package github

import (
	"fmt"
	"log/slog"
)

const redacted = "[REDACTED]"

// redactedValue は、資格情報をログとエラー本文に出さないため、fmt、slog、JSON のどの出力でも値を伏せる。
// 資格情報の型は、このフィールドを埋め込んで伏せ方を共有する。
// 値を取り出す expose を非公開にし、GitHub Client の外へ値を出す口を、資格情報の型ごとに設けるかどうか決められるようにする。
type redactedValue struct {
	// fmt はほかの struct の非公開フィールドに置かれた値の Format を呼べず、フィールドをそのまま出す。
	// ポインタで持ち、そのときにもアドレスしか出ないようにする
	value *string
}

// expose は、GitHub API の認証に使う値を返す。ゼロ値で呼ぶと panic する。
func (v redactedValue) expose() string {
	return *v.value
}

// Format は、どの書式指定でも伏せた表記を出す。Formatter は String と GoString より優先されるため、これだけで足りる。
func (v redactedValue) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(redacted)) // fmt.State の Write は fmt の内部バッファに追記するだけで、error を返さない
}

// LogValue は、slog の attribute として伏せた表記を返す。
func (v redactedValue) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON は、伏せた表記を JSON の文字列として返す。encoding/json と encoding/json/v2 のどちらもこのメソッドを使う。
func (v redactedValue) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redacted + `"`), nil
}
