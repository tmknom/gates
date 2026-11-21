package permission

import "testing"

//trace:checks AC-AUTHZ-009
func TestIncludesByLevel(t *testing.T) {
	tests := []struct {
		name      string
		limit     []entry
		requested entry
		includes  bool
	}{
		{name: "read is included by write", limit: []entry{{"contents", "write"}}, requested: entry{"contents", "read"}, includes: true},
		{name: "write is not included by read", limit: []entry{{"contents", "read"}}, requested: entry{"contents", "write"}, includes: false},
		{name: "higher level of another permission name does not include", limit: []entry{{"issues", "write"}}, requested: entry{"contents", "read"}, includes: false},
		{name: "higher of two levels of the same permission name includes", limit: []entry{{"contents", "read"}, {"contents", "write"}}, requested: entry{"contents", "write"}, includes: true},
		// 本物の列挙には write だけを持つ権限名がないため、許可権限にない権限名で、水準の比較だけでは含まれる組を作る
		{name: "entry outside allowed permissions is not included by a higher level", limit: []entry{{"members", "write"}}, requested: entry{"members", "read"}, includes: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := make(map[entry]bool, len(tt.limit))
			for _, e := range tt.limit {
				limit[e] = true
			}
			if got := includesByLevel(limit, tt.requested); got != tt.includes {
				t.Errorf("includesByLevel(%v, %v) = %v, want %v", tt.limit, tt.requested, got, tt.includes)
			}
		})
	}
}

func TestLevelAtLeast(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		other   string
		atLeast bool
	}{
		{name: "same level returns true", level: "write", other: "write", atLeast: true},
		{name: "write returns true against read", level: "write", other: "read", atLeast: true},
		{name: "admin returns true against write", level: "admin", other: "write", atLeast: true},
		{name: "write returns false against admin", level: "write", other: "admin", atLeast: false},
		{name: "level outside level order returns false against an ordered level", level: "maintain", other: "read", atLeast: false},
		{name: "ordered level returns false against a level outside level order", level: "admin", other: "maintain", atLeast: false},
		{name: "same level outside level order returns false", level: "maintain", other: "maintain", atLeast: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := levelAtLeast(tt.level, tt.other); got != tt.atLeast {
				t.Errorf("levelAtLeast(%q, %q) = %v, want %v", tt.level, tt.other, got, tt.atLeast)
			}
		})
	}
}
