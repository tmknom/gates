// Package credential は、資格情報の型が共有する、値を伏せて出力する仕組みを提供する。
package credential

import (
	"fmt"
	"log/slog"
)

const redacted = "[REDACTED]"

// Redacted は、資格情報をログとエラー本文に出さないため、資格情報の型に埋め込まれ、fmt、slog、JSON のどの出力でも伏せた表記を出す。
// 値を持たず取り出す口もないため、値を取り出せるかどうかは、埋め込んだ資格情報の型の package が資格情報ごとに決める。
// 埋め込む型は値をポインタで持つ。ほかの struct の非公開フィールドに置かれると、fmt は Format を呼ばずにフィールドをそのまま出すため。
type Redacted struct{}

// Format は、どの書式指定でも伏せた表記を出す。Formatter は String と GoString より優先されるため、これだけで足りる。
func (Redacted) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(redacted)) // fmt.State の Write は fmt の内部バッファに追記するだけで、error を返さない
}

// LogValue は、slog の attribute として伏せた表記を返す。
func (Redacted) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON は、伏せた表記を JSON の文字列として返す。encoding/json と encoding/json/v2 のどちらも、埋め込んだ型に昇格したこのメソッドを使う。
func (Redacted) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redacted + `"`), nil
}
