package cmd

import (
	"fmt"
	"io"
	"time"
)

// Env は、CLI が依存する実行環境。入出力と時刻を外から注入し、テストで差し替えられるようにする。
type Env struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Current time.Time
	WorkDir string
	Version string
	Commit  string
}

const devVersion = "dev"

// version は --version で表示する文字列を返す。
// cobra は Version が空だと --version フラグを登録しないため、空の場合は devVersion で埋める。
func (e Env) version() string {
	v := e.Version
	if v == "" {
		v = devVersion
	}
	if e.Commit == "" {
		return v
	}
	return fmt.Sprintf("%s (%s)", v, e.Commit)
}
