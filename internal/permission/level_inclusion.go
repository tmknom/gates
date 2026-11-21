package permission

import "slices"

// levelOrder は、水準の順序である。低い水準から並べ、ここにない水準はどの水準とも比べない。
var levelOrder = []string{"read", "write", "admin"}

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
