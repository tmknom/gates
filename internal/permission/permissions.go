package permission

import (
	"fmt"
	"maps"
)

// maxTermLength の根拠は Lambda Design の「権限名と水準の文字種」に記述する。
const maxTermLength = 64

// errMalformedTerm は、権限名か水準が文字種を満たさないことを表す。文言に入力の値を含めない。
var errMalformedTerm = fmt.Errorf("must be 1 to %d lowercase ASCII letters or '_'", maxTermLength)

// Permissions は、権限名と水準の組（Permission Entry）の集合である。
// GitHub の permissions オブジェクトが権限名をキーに持つため、権限名ごとに水準を一つ持つ。
// map を持つため == と map のキーに使えず、比較は Equal だけを使う。
type Permissions struct {
	levels map[string]string
}

// NewPermissions は、空でないことと、権限名と水準がどれも ASCII 小文字とアンダースコアからなる maxTermLength 文字以下の文字列であることを検証する。
// 文字種の根拠は Lambda Design の「権限名と水準の文字種」に記述する。
// 許可権限かどうかは検証しない。形式検証と認可の分担は Security Design の「Authorization Rule」に従う。
func NewPermissions(levels map[string]string) (Permissions, error) {
	if len(levels) == 0 {
		return Permissions{}, fmt.Errorf("%w: must not be empty", ErrMalformedPermissions)
	}
	// map の反復順は一定でないため、権限名をすべて調べてから水準を調べ、両方が不正なときの文言を一つに定める
	for name := range levels {
		if !wellFormedTerm(name) {
			return Permissions{}, fmt.Errorf("%w: permission name %w", ErrMalformedPermissions, errMalformedTerm)
		}
	}
	for _, level := range levels {
		if !wellFormedTerm(level) {
			return Permissions{}, fmt.Errorf("%w: permission level %w", ErrMalformedPermissions, errMalformedTerm)
		}
	}
	// 呼び出し元が後から map を書き換えても、検証済みの値が変わらないように写す
	return Permissions{levels: maps.Clone(levels)}, nil
}

// AllowedPermissions は、GATES が発行を許可する Permissions を返す。
// Go では struct を定数にできず、公開変数は import 側から差し替えられるため、呼ぶたびに値を作る関数にする。
func AllowedPermissions() Permissions {
	return Permissions{levels: map[string]string{"contents": "read"}}
}

func wellFormedTerm(s string) bool {
	if s == "" || len(s) > maxTermLength {
		return false
	}
	for i := range len(s) {
		if (s[i] < 'a' || 'z' < s[i]) && s[i] != '_' {
			return false
		}
	}
	return true
}

// Equal は、Permission Entry の集合として等しいかを返す。
func (p Permissions) Equal(other Permissions) bool {
	return maps.Equal(p.levels, other.levels)
}
