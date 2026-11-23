package strictyaml

import (
	"slices"
	"testing"
)

func TestReadRepositoryID(t *testing.T) {
	var c Conditions
	got, ok := ReadRepositoryID(value(t, "v: \"100\"\n"), "p", &c)
	if !ok || got.Int64() != 100 || len(c.List()) != 0 {
		t.Errorf("ReadRepositoryID = %d, %v with conditions %q, want 100", got.Int64(), ok, c.List())
	}
}

func TestReadRepositoryID_Invalid(t *testing.T) {
	const idFormat = "p must be a positive decimal integer without a sign or leading zeros that fits in int64"
	tests := []struct {
		name     string
		document string
		want     string
	}{
		{name: "unquoted id is not read", document: "v: 100\n", want: "p must be a quoted string"},
		{name: "zero id", document: "v: \"0\"\n", want: idFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			if _, ok := ReadRepositoryID(value(t, tt.document), "p", &c); ok {
				t.Error("ReadRepositoryID ok = true, want false")
			}
			if want := []string{tt.want}; !slices.Equal(c.List(), want) {
				t.Errorf("conditions = %q, want %q", c.List(), want)
			}
		})
	}
}

func TestReadRepositoryName(t *testing.T) {
	var c Conditions
	got, ok := ReadRepositoryName(value(t, "v: deploy-tools\n"), "p", &c)
	if !ok || got != "deploy-tools" || len(c.List()) != 0 {
		t.Errorf("ReadRepositoryName = %q, %v with conditions %q, want %q", got, ok, c.List(), "deploy-tools")
	}
}

func TestReadRepositoryName_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     string
	}{
		{name: "integer is not a string", document: "v: 1\n", want: "p must be a string"},
		{name: "name with owner", document: "v: secret/value\n", want: "p must consist of ASCII letters, digits, '-', '_', or '.'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			if _, ok := ReadRepositoryName(value(t, tt.document), "p", &c); ok {
				t.Error("ReadRepositoryName ok = true, want false")
			}
			if want := []string{tt.want}; !slices.Equal(c.List(), want) {
				t.Errorf("conditions = %q, want %q", c.List(), want)
			}
		})
	}
}

func TestReadLimit(t *testing.T) {
	var c Conditions
	limit := ReadLimit(value(t, "v: {contents: write, 'pull_requests': \"read\"}\n"), "p", &c)
	if len(c.List()) != 0 {
		t.Fatalf("conditions = %q, want none", c.List())
	}
	if !limit.Contains(mustNewPermissions(t, map[string]string{"contents": "write", "pull_requests": "read"})) {
		t.Error("limit does not contain contents:write and pull_requests:read")
	}
	if limit.Contains(mustNewPermissions(t, map[string]string{"pull_requests": "write"})) {
		t.Error("limit contains pull_requests:write")
	}
}

// 権限の上限は書かれた順の組の位置で誤りを示すため、YAML として読めない組があれば作らず、残りの組の誤りを報告しない
func TestReadLimit_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     []string
	}{
		{name: "sequence is not a mapping", document: "v: []\n", want: []string{"p must be a mapping"}},
		{name: "empty permissions", document: "v: {}\n", want: []string{"p: invalid permission limit: must have at least one permission entry"}},
		{name: "duplicate permission name differing only in quotes", document: "v: {contents: read, \"contents\": write}\n", want: []string{"p: invalid permission limit: entries[1] is a duplicate permission name"}},
		{name: "value that is not a string", document: "v: {contents: 1, issues: write, pull_requests: READ}\n", want: []string{"p has a value that is not a string"}},
		{name: "alias value", document: "a: &l write\nv: {contents: *l, issues: write}\n", want: []string{"line 2: must not be an alias"}},
		{name: "key that is not a string", document: "v: {1: write, issues: write}\n", want: []string{"p has a key that is not a string"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Conditions
			limit := ReadLimit(value(t, tt.document), "p", &c)
			if !slices.Equal(c.List(), tt.want) {
				t.Errorf("conditions = %q, want %q", c.List(), tt.want)
			}
			if limit.Contains(mustNewPermissions(t, map[string]string{"issues": "write"})) {
				t.Error("limit was built from the readable pairs")
			}
		})
	}
}

func TestReadLimit_MissingValueHasNoCondition(t *testing.T) {
	var c Conditions
	ReadLimit(Node{}, "p", &c)
	if len(c.List()) != 0 {
		t.Errorf("conditions = %q, want none", c.List())
	}
}
