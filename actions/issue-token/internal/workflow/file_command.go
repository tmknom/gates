package workflow

import (
	"errors"
	"os"
	"strings"
)

// ErrMultilineValue は、ファイルコマンドの値が改行を含むことを表す。
var ErrMultilineValue = errors.New("value must not contain a line break")

// FileCommand は、ファイルコマンドに書く name=value の 1 行である。
type FileCommand struct {
	Name, Value string
}

// AppendFileCommand は、path のファイルに commands の name=value の行を順に追記する。
// 行をまとめて 1 回で書くのは、途中で書けなくなったときに、一部の行だけが残らないようにするため。
// 改行を含む値は 1 行に書けず、区切りの指定が要る別の形式になるため、1 つでもあれば何も書かずに拒否する。
// ファイルは Runner が作るため、なければ作らずに error を返す。
// 値は Runner が生のまま読むため、ワークフローコマンドと違い符号化しない。
func AppendFileCommand(path string, commands ...FileCommand) error {
	var lines strings.Builder
	for _, command := range commands {
		if strings.ContainsAny(command.Value, "\r\n") {
			return ErrMultilineValue
		}
		lines.WriteString(command.Name + "=" + command.Value + "\n")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, writeErr := f.WriteString(lines.String())
	return errors.Join(writeErr, f.Close())
}
