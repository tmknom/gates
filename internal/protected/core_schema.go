package protected

import "regexp"

// coreSchemaNonString は、YAML 1.2.2 の Core Schema の「Tag Resolution」の表のうち、str 以外に解決する正規表現をまとめたものである。
// YAML のライブラリの型の解決は Core Schema と食い違うため使わない。理由は Lambda Design の「保護対象の一覧の書式」に記述する。
var coreSchemaNonString = regexp.MustCompile(`^(?:` +
	`|null|Null|NULL|~|` +
	`true|True|TRUE|false|False|FALSE|` +
	`[-+]?[0-9]+|0o[0-7]+|0x[0-9a-fA-F]+|` +
	`[-+]?(?:\.[0-9]+|[0-9]+(?:\.[0-9]*)?)(?:[eE][-+]?[0-9]+)?|` +
	`[-+]?(?:\.inf|\.Inf|\.INF)|\.nan|\.NaN|\.NAN` +
	`)$`)

// resolvesToNonString は、引用符のないスカラーの値が、Core Schema で null、bool、int、float のどれかに解決されるかを返す。
// 空の値は null に解決される。
func resolvesToNonString(plain string) bool {
	return coreSchemaNonString.MatchString(plain)
}
