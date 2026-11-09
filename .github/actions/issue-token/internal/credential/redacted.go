// Package credential は、Docker Action が扱う資格情報の型を提供する。
package credential

import "fmt"

const redactedNotation = "[REDACTED]"

// redacted は、資格情報の型に埋め込まれ、fmt と JSON のどの出力でも伏せた表記を出す。
// 埋め込む型は値をポインタで持つ。ほかの struct の非公開フィールドに置かれると、fmt は Format を呼ばずにフィールドをそのまま出すため。
type redacted struct{}

// Format は、どの書式指定でも伏せた表記を出す。Formatter は String と GoString より優先されるため、これだけで足りる。
func (redacted) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(redactedNotation)) // fmt.State の Write は fmt の内部バッファに追記するだけで、error を返さない
}

// MarshalJSON は、伏せた表記を JSON の文字列として返す。encoding/json と encoding/json/v2 のどちらも、埋め込んだ型に昇格したこのメソッドを使う。
func (redacted) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redactedNotation + `"`), nil
}
