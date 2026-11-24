package permission

import (
	"fmt"
	"iter"
	"maps"
	"slices"
)

// Permissions は、権限名と水準の組（Permission Entry）の集合である。
// GitHub の permissions オブジェクトが権限名をキーに持つため、権限名ごとに水準を一つ持つ。
// map を持つため == と map のキーに使えず、比較は Equal だけを使う。
type Permissions struct {
	levels map[string]string
}

// impliedPermissions は、要求にかかわらず GitHub が付与する Permission Entry のうち、GATES が許容するもの（付随権限）である。
// 付随権限を `metadata:read` だけにする理由は Security Design の「Issuance Result Verification」に記述する。
var impliedPermissions = map[entry]bool{{name: "metadata", level: "read"}: true}

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

// PolicyRead は、ポリシー読み取り許可の Permissions（single_file:read だけ）を返す。
// 要求から作らない固定の値のため、検証を経ずに作る。
func PolicyRead() Permissions {
	return Permissions{levels: map[string]string{"single_file": "read"}}
}

// WithinAllowed は、Permission Entry がどれも許可権限に含まれるかを返す。
// 組で照合し、許可権限が同じ権限名の別の水準を含んでも、その組は含まれない。
// ゼロ値は許可する要求を表さないため、含まれないとする。
func (p Permissions) WithinAllowed() bool {
	if len(p.levels) == 0 {
		return false
	}
	for name, level := range p.levels {
		if !allowedPermissions[entry{name: name, level: level}] {
			return false
		}
	}
	return true
}

// EqualsExceptImplied は、INV-012 の実効権限の条件に従い、レシーバの実効権限が発行許可の Permissions をすべて同じ水準で含み、それ以外は付随権限だけかを返す。
// 発行許可が付随権限の権限名を含むときは、その水準も発行許可に照合する。
// 発行許可のゼロ値は発行 API を呼ぶ前に止まる値だが、付随権限だけの実効権限を受理しないよう、含まないとする。
func (p Permissions) EqualsExceptImplied(granted Permissions) bool {
	if len(granted.levels) == 0 {
		return false
	}
	for name, level := range granted.levels {
		if p.levels[name] != level {
			return false
		}
	}
	for name, level := range p.levels {
		if granted.levels[name] != level && !impliedPermissions[entry{name: name, level: level}] {
			return false
		}
	}
	return true
}

// Entries は、Permission Entry を権限名と水準の組として返す。
// map の反復順は一定でないため、反復順を決定的にするよう、権限名の昇順に並べる。
func (p Permissions) Entries() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for _, name := range slices.Sorted(maps.Keys(p.levels)) {
			if !yield(name, p.levels[name]) {
				return
			}
		}
	}
}

// Equal は、Permission Entry の集合として等しいかを返す。
func (p Permissions) Equal(other Permissions) bool {
	return maps.Equal(p.levels, other.levels)
}
