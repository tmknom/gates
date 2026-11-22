package permission

import "slices"

// levelOrder は、水準の順序である。低い水準から並べ、ここにない水準はどの水準とも比べない。
var levelOrder = []string{"read", "write", "admin"}

// containsByLevel は、Permissions の Permission Entry がどれも上限に水準の包含で含まれるかを返す。
// ゼロ値の Permissions は許可する要求を表さないため、含まれないとする。
func containsByLevel(limit map[entry]bool, p Permissions) bool {
	if len(p.levels) == 0 {
		return false
	}
	for name, level := range p.levels {
		if !includesByLevel(limit, entry{name: name, level: level}) {
			return false
		}
	}
	return true
}

// includesByLevel は、上限が要求の Permission Entry を水準の包含で含むかを返す。
// 許可権限に含まれることを自らの条件に持つ理由は、Domain Model の「Permission Levels」に記述する。
func includesByLevel(limit map[entry]bool, requested entry) bool {
	if !allowedPermissions[requested] {
		return false
	}
	for e := range limit {
		if e.name == requested.name && levelAtLeast(e.level, requested.level) {
			return true
		}
	}
	return false
}

// levelAtLeast は、水準の順序で level が other と同じかより高いかを返す。どちらかが順序にない水準なら false を返す。
func levelAtLeast(level, other string) bool {
	at, otherAt := slices.Index(levelOrder, level), slices.Index(levelOrder, other)
	return otherAt >= 0 && at >= otherAt
}
