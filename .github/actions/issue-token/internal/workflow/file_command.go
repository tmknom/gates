package workflow

import (
	"errors"
	"os"
	"strings"
)

// ErrMultilineValue は、ファイルコマンドの値が改行を含むことを表す。
var ErrMultilineValue = errors.New("value must not contain a line break")

// AppendFileCommand は、path のファイルに name=value の 1 行を追記する。
// 改行を含む値は 1 行に書けず、区切りの指定が要る別の形式になるため、何も書かずに拒否する。
// ファイルは Runner が作るため、なければ作らずに error を返す。
// 値は Runner が生のまま読むため、ワークフローコマンドと違い符号化しない。
func AppendFileCommand(path, name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return ErrMultilineValue
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, writeErr := f.WriteString(name + "=" + value + "\n")
	return errors.Join(writeErr, f.Close())
}
