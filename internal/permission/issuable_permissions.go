package permission

import (
	"fmt"
	"strings"
)

// IssuablePermissions は、許可権限のうち、そのデプロイで発行してよい Permission Entry の集合（発行上限）である。
// 一つの権限名に複数の水準を含み得るため、Permissions ではなく組の集合で持つ。
// フィールドを非公開にし、ゼロ値以外は NewIssuablePermissions でしか作れないようにする。
type IssuablePermissions struct {
	entries map[entry]bool
}

// NewIssuablePermissions は、権限名と水準を `:` でつないだ組の表記の列を読み、INV-015 を検証して発行上限を作る。
// 前後の空白を除かず、重複を一つにまとめずに拒否し、すべての組の誤りを位置とともに返す理由は Lambda Design の「Runtime Entry」に記述する。
// 文言には組の位置と条件だけを書き、権限名と水準を含めない。
// 組ごとの誤りを errors.Join の改行ではなく 1 行につなぐのは、固定設定の読み込みが環境変数ごとの誤りを改行で並べ、2 行目以降がどの環境変数の誤りか読めなくなるため。
func NewIssuablePermissions(notations []string) (IssuablePermissions, error) {
	if len(notations) == 0 {
		return IssuablePermissions{}, fmt.Errorf("%w: must have at least one permission entry", ErrInvalidIssuablePermissions)
	}
	entries := make(map[entry]bool, len(notations))
	var conditions []string
	for i, notation := range notations {
		e, err := parseEntry(notation)
		switch {
		case err != nil:
			conditions = append(conditions, fmt.Sprintf("entries[%d] %v", i, err))
		case entries[e]:
			conditions = append(conditions, fmt.Sprintf("entries[%d] is a duplicate", i))
		case !allowedPermissions[e]:
			conditions = append(conditions, fmt.Sprintf("entries[%d] is not in allowed permissions", i))
		}
		entries[e] = true
	}
	if len(conditions) > 0 {
		return IssuablePermissions{}, fmt.Errorf("%w: %s", ErrInvalidIssuablePermissions, strings.Join(conditions, "; "))
	}
	return IssuablePermissions{entries: entries}, nil
}

// IsZero は、NewIssuablePermissions で作られていないゼロ値かを返す。
func (i IssuablePermissions) IsZero() bool {
	return i.entries == nil
}

// Contains は、Permissions の Permission Entry がどれも発行上限に水準の包含で含まれるかを返す。
func (i IssuablePermissions) Contains(p Permissions) bool {
	return containsByLevel(i.entries, p)
}
