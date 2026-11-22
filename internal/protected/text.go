package protected

import (
	"strings"
	"unicode/utf8"
)

// scanText は、YAML として解析する前の文字列で、文字の符号化の条件と、構文木に残らない YAML の機能（ディレクティブと明示的なタグ）を確かめる。
// UTF-8 として正しくなければ、以降の判定の位置が意味を持たないため false を返し、ほかの条件は集めて true を返す。
//
// 見分け方と、それが正しい値を拒否しない理由は Lambda Design の「保護対象の一覧の書式」に記述する。
// 引用符の中の " #" をコメントの始まりと読み違えて ! を見逃すのは、引用符の中に空白と # を含む値を持つ文書で、どの値の文字種にも合わずに別の理由で拒否される。
func scanText(document string, c *conditions) bool {
	if !utf8.ValidString(document) {
		c.add("must be valid UTF-8")
		return false
	}
	if strings.ContainsRune(document, '\uFEFF') {
		c.add("must not contain a byte order mark")
	}
	if strings.ContainsAny(document, "\u0085\u2028\u2029") {
		c.add("must not contain NEL, LS, or PS")
	}
	for i, line := range lines(document) {
		// %TAG のディレクティブは ! を含むため、ディレクティブの行はタグとして重ねて報告しない
		switch {
		case strings.HasPrefix(line, "%"):
			c.add("line %d: must not have a directive", i+1)
		case strings.Contains(withoutComment(line), "!"):
			c.add("line %d: must not have an explicit tag", i+1)
		}
	}
	return true
}

// lines は、YAML 1.2.2 の改行（CRLF、CR、LF）で行に分ける。NEL、LS、PS は改行として扱わず、文字の符号化の条件で拒否する。
func lines(document string) []string {
	return strings.Split(strings.ReplaceAll(strings.ReplaceAll(document, "\r\n", "\n"), "\r", "\n"), "\n")
}

// withoutComment は、行のうちコメントより前の部分を返す。YAML のコメントは、行頭か空白の直後の # から始まる。
func withoutComment(line string) string {
	for i := range len(line) {
		if line[i] == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
			return line[:i]
		}
	}
	return line
}
