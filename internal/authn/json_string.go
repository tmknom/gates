package authn

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// stringValue は JSON 文字列の値を返し、値がないか文字列でなければ空文字列を返す。
func stringValue(value jsontext.Value) string {
	var s string
	if value.Kind() != '"' || json.Unmarshal(value, &s) != nil {
		return ""
	}
	return s
}
