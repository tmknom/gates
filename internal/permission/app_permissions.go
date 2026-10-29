package permission

import (
	"fmt"
	"maps"
)

// maxTermLength の根拠は Lambda Design の「権限名と水準の文字種」に記述する。
const maxTermLength = 64

// errMalformedTerm は、権限名か水準が文字種を満たさないことを表す。文言に入力の値を含めない。
var errMalformedTerm = fmt.Errorf("must be 1 to %d lowercase ASCII letters or '_'", maxTermLength)

// AppPermissions は、権限名と水準の組（Permission）の集合である。
// GitHub の permissions オブジェクトが権限名をキーに持つため、権限名ごとに水準を一つ持つ。
// map を持つため == と map のキーに使えず、比較は Equal だけを使う。
type AppPermissions struct {
	levels map[string]string
}

// NewAppPermissions は、空でないことと、権限名と水準がどれも ASCII 小文字とアンダースコアからなる maxTermLength 文字以下の文字列であることを検証する。
// 文字種の根拠は Lambda Design の「権限名と水準の文字種」に記述する。
// 許可権限かどうかは検証しない。形式検証と認可の分担は Security Design の「Authorization Rule」に従う。
func NewAppPermissions(levels map[string]string) (AppPermissions, error) {
	if len(levels) == 0 {
		return AppPermissions{}, fmt.Errorf("%w: must not be empty", ErrInvalidAppPermissions)
	}
	// map の反復順は一定でないため、権限名をすべて調べてから水準を調べ、両方が不正なときの文言を一つに定める
	for name := range levels {
		if !wellFormedTerm(name) {
			return AppPermissions{}, fmt.Errorf("%w: permission name %w", ErrInvalidAppPermissions, errMalformedTerm)
		}
	}
	for _, level := range levels {
		if !wellFormedTerm(level) {
			return AppPermissions{}, fmt.Errorf("%w: permission level %w", ErrInvalidAppPermissions, errMalformedTerm)
		}
	}
	// 呼び出し元が後から map を書き換えても、検証済みの値が変わらないように写す
	return AppPermissions{levels: maps.Clone(levels)}, nil
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

// Equal は、権限名と水準の組の集合として等しいかを返す。
func (p AppPermissions) Equal(other AppPermissions) bool {
	return maps.Equal(p.levels, other.levels)
}
