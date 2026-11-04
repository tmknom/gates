package config

import "errors"

// ErrInvalidConfig の文言には、環境変数の名前と満たさなかった条件だけを書き、値を書かない。理由は Lambda Design の「Runtime Entry」に記述する。
var ErrInvalidConfig = errors.New("invalid config")
