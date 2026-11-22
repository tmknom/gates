package permission

import (
	"fmt"
	"strings"
)

// Limit は、許可 Caller に許す Permission Entry の上限（権限の上限）である。
// Permissions と同じく権限名ごとに水準を一つ持つが、照合を発行上限と同じ水準の包含で行うため、組の集合で持つ。
// フィールドを非公開にし、ゼロ値以外は NewLimit でしか作れないようにする。
type Limit struct {
	entries map[entry]bool
}

// LimitEntry は、権限の上限に書かれた、検証する前の権限名と水準の組である。
type LimitEntry struct {
	Name  string
	Level string
}

// NewLimit は、書かれた順の組を読み、INV-016 が権限の上限に求める条件を検証して権限の上限を作る。
// 誤りは組の位置と条件だけで示し、権限名と水準を含めない。組ごとの誤りを 1 行につなぐ理由は NewIssuablePermissions と同じである。
func NewLimit(written []LimitEntry) (Limit, error) {
	if len(written) == 0 {
		return Limit{}, fmt.Errorf("%w: must have at least one permission entry", ErrInvalidLimit)
	}
	entries := make(map[entry]bool, len(written))
	names := make(map[string]bool, len(written))
	var conditions []string
	for i, w := range written {
		name, level := w.Name, w.Level
		e := entry{name: name, level: level}
		switch {
		case !wellFormedTerm(name):
			conditions = append(conditions, fmt.Sprintf("entries[%d] permission name %v", i, errMalformedTerm))
		case !wellFormedTerm(level):
			conditions = append(conditions, fmt.Sprintf("entries[%d] permission level %v", i, errMalformedTerm))
		case names[name]:
			conditions = append(conditions, fmt.Sprintf("entries[%d] is a duplicate permission name", i))
		case !allowedPermissions[e]:
			conditions = append(conditions, fmt.Sprintf("entries[%d] is not in allowed permissions", i))
		}
		names[name] = true
		entries[e] = true
	}
	if len(conditions) > 0 {
		return Limit{}, fmt.Errorf("%w: %s", ErrInvalidLimit, strings.Join(conditions, "; "))
	}
	return Limit{entries: entries}, nil
}

// Contains は、Permissions の Permission Entry がどれも権限の上限に水準の包含で含まれるかを返す。
// 付随権限を足して照合しない理由は Security Design の「Permission Boundary」に記述する。
func (l Limit) Contains(p Permissions) bool {
	return containsByLevel(l.entries, p)
}
