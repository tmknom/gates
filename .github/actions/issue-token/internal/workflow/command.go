// Package workflow は、Runner が読むワークフローコマンドとファイルコマンドを書く。
package workflow

import (
	"io"
	"strings"
)

// credential は、マスクする値を取り出す口を workflow の中に閉じる。
// 呼び出し側が資格情報の値を取り出さずにマスクできるようにするため。
type credential interface{ Expose() string }

// AddMask は、値をマスクするワークフローコマンドを書く。
func AddMask(w io.Writer, value credential) error {
	return writeCommand(w, "add-mask", value.Expose())
}

// Error は、error の注釈を書く。
func Error(w io.Writer, message string) error {
	return writeCommand(w, "error", message)
}

// Warning は、warning の注釈を書く。
func Warning(w io.Writer, message string) error {
	return writeCommand(w, "warning", message)
}

func writeCommand(w io.Writer, command, data string) error {
	_, err := io.WriteString(w, "::"+command+"::"+escapeData(data)+"\n")
	return err
}

// escapeData は、%、CR、LF を Runner の復号に合わせて符号化する。
// 1 回の走査で置換するため、符号化で出た % を再び符号化することがない。
func escapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}
