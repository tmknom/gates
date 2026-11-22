package response

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"
)

// legacyCommandPrefix は、Runner が行の途中からも読む旧形式のワークフローコマンドの接頭辞である（actions/runner の ActionCommand.TryParse）。
const legacyCommandPrefix = "##["

// stringOf は、JSON の文字列の値を返す。文字列でなければ false を返す。
func stringOf(value jsontext.Value) (string, bool) {
	if value.Kind() != '"' {
		return "", false
	}
	var s string
	// 値は全体の復号を通った JSON の文字列のため、復号は失敗しない
	_ = json.Unmarshal(value, &s)
	return s, true
}

// displayable は、値を失敗の文言に含めてよいかを返す。
// 失敗の文言も注釈としてログの行に出すため、usableItem と同じく旧形式のワークフローコマンドの接頭辞を拒む。
// 改行は注釈の符号化と %q が 1 行に収めるため、拒まない。
func displayable(s string) bool {
	return !strings.Contains(s, legacyCommandPrefix)
}

// displayString は、項目が文字列で displayable を満たすときだけ、その値を返す。
func displayString(value jsontext.Value) (string, bool) {
	s, ok := stringOf(value)
	if !ok || !displayable(s) {
		return "", false
	}
	return s, true
}

// requestIDSuffix は、request_id が displayString を満たすときだけ、文言に添える括弧の部分を返す。
func requestIDSuffix(requestID jsontext.Value) string {
	id, ok := displayString(requestID)
	if !ok {
		return ""
	}
	return fmt.Sprintf(" (request id %q)", id)
}
