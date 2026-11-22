package protected

import (
	"slices"
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

func TestReadDocument(t *testing.T) {
	got, conditions := read(t, validDocument)
	if len(conditions) != 0 {
		t.Fatalf("conditions = %q, want none", conditions)
	}
	if len(got) != 2 {
		t.Fatalf("len(repositories) = %d, want 2", len(got))
	}
	first, second := got[0], got[1]
	if first.id.Int64() != 200 || first.name != "platform-infra" || len(first.callers) != 1 {
		t.Errorf("repositories[0] = %d %q with %d callers, want 200 %q with 1 caller", first.id.Int64(), first.name, len(first.callers), "platform-infra")
	}
	if second.id.Int64() != 201 || second.name != "audit-records" || len(second.callers) != 0 {
		t.Errorf("repositories[1] = %d %q with %d callers, want 201 %q with no caller", second.id.Int64(), second.name, len(second.callers), "audit-records")
	}
	caller := first.callers[0]
	if caller.repositoryID.Int64() != 100 || caller.repositoryName != "deploy-tools" {
		t.Errorf("callers[0] = %d %q, want 100 %q", caller.repositoryID.Int64(), caller.repositoryName, "deploy-tools")
	}
	if !caller.limit.Contains(mustNewPermissions(t, map[string]string{"contents": "write", "pull_requests": "read"})) {
		t.Error("callers[0].limit does not contain contents:write and pull_requests:read")
	}
	if caller.limit.Contains(mustNewPermissions(t, map[string]string{"pull_requests": "write"})) {
		t.Error("callers[0].limit contains pull_requests:write")
	}
}

func TestReadDocument_Accepted(t *testing.T) {
	tests := []struct {
		name     string
		document string
	}{
		{name: "empty repositories are accepted", document: "repositories: []\n"},
		{name: "flow style is accepted", document: `{repositories: [{id: "1", name: a, callers: [{repository_id: "2", repository_name: b, permissions: {contents: read}}]}]}`},
		{name: "quoted names and permissions are accepted", document: replaceOnce(t, replaceOnce(t, validDocument, "name: platform-infra", `name: "platform-infra"`), "contents: write", `'contents': "write"`)},
		{name: "double-quoted digits are a string name", document: replaceOnce(t, validDocument, "name: platform-infra", `name: "123"`)},
		{name: "single-quoted boolean is a string name", document: replaceOnce(t, validDocument, "name: platform-infra", `name: 'true'`)},
		{name: "single-quoted id is accepted", document: replaceOnce(t, validDocument, `id: "200"`, `id: '200'`)},
		{name: "escaped id is read after unescaping", document: replaceOnce(t, validDocument, `id: "200"`, `id: "\x32\x30\x30"`)},
		{name: "names that YAML 1.1 reads as booleans are strings", document: replaceOnce(t, validDocument, "name: platform-infra", "name: yes")},
		{name: "names that the YAML library reads as non-strings are strings", document: replaceOnce(t, validDocument, "name: platform-infra", "name: 2001-12-14")},
		{name: "literal block scalar name is a string", document: replaceOnce(t, validDocument, "name: platform-infra", "name: |-\n      123")},
		{name: "folded block scalar name is a string", document: replaceOnce(t, validDocument, "name: platform-infra", "name: >-\n      true")},
		{name: "same allowed caller in another protected repository is accepted", document: replaceOnce(t, validDocument, "callers: []", "callers: [{repository_id: \"100\", repository_name: deploy-tools, permissions: {contents: read}}]")},
		{name: "same allowed caller id with another name in another protected repository is accepted", document: replaceOnce(t, validDocument, "callers: []", "callers: [{repository_id: \"100\", repository_name: renamed-tools, permissions: {contents: read}}]")},
		{name: "protected repository allowing itself is accepted", document: replaceOnce(t, validDocument, "callers: []", "callers: [{repository_id: \"201\", repository_name: audit-records, permissions: {contents: read}}]")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, conditions := read(t, tt.document); len(conditions) != 0 {
				t.Errorf("conditions = %q, want none", conditions)
			}
		})
	}
}

//trace:checks AC-AUTHZ-013
func TestReadDocument_Invalid(t *testing.T) {
	const (
		repository = "repositories[0]"
		caller     = "repositories[0].callers[0]"
		limit      = "repositories[0].callers[0].permissions"
		idFormat   = " must be a positive decimal integer without a sign or leading zeros that fits in int64"
		nameChars  = " must consist of ASCII letters, digits, '-', '_', or '.'"
		notAllowed = ": invalid permission limit: entries[0] is not in allowed permissions"
	)
	tests := []struct {
		name     string
		old, new string
		want     []string
	}{
		// 文書
		{name: "sequence document is not a mapping", old: validDocument, new: "- a\n", want: []string{"document must be a mapping"}},
		{name: "scalar document is not a mapping", old: validDocument, new: "repositories\n", want: []string{"document must be a mapping"}},
		{name: "unknown top-level version key", old: "repositories:\n", new: "version: \"1\"\nrepositories:\n", want: []string{"document has an unknown key"}},
		{name: "missing repositories", old: validDocument, new: "{}\n", want: []string{"repositories is required"}},
		{name: "duplicate repositories key", old: "repositories:\n", new: "repositories: []\nrepositories:\n", want: []string{"repositories is a duplicate key"}},
		{name: "duplicate repositories key differing only in quotes", old: "repositories:\n", new: "repositories: []\n\"repositories\":\n", want: []string{"repositories is a duplicate key"}},
		{name: "repositories mapping is not a sequence", old: validDocument, new: "repositories: {}\n", want: []string{"repositories must be a sequence"}},
		{name: "repositories scalar is not a sequence", old: validDocument, new: "repositories: a\n", want: []string{"repositories must be a sequence"}},
		{name: "null repositories is not a sequence", old: validDocument, new: "repositories: ~\n", want: []string{"repositories must be a sequence"}},
		{name: "empty repositories value is not a sequence", old: validDocument, new: "repositories:\n", want: []string{"repositories must be a sequence"}},
		// 保護対象
		{name: "scalar protected repository is not a mapping", old: validDocument, new: "repositories: [a]\n", want: []string{repository + " must be a mapping"}},
		{name: "sequence protected repository is not a mapping", old: validDocument, new: "repositories: [[]]\n", want: []string{repository + " must be a mapping"}},
		{name: "unknown key in protected repository", old: "    name: platform-infra\n", new: "    name: platform-infra\n    owner: example\n", want: []string{repository + " has an unknown key"}},
		{name: "missing id", old: "  - id: \"200\"\n    name", new: "  - name", want: []string{repository + ".id is required"}},
		{name: "missing name", old: "    name: platform-infra\n", new: "", want: []string{repository + ".name is required"}},
		{name: "missing callers", old: "    name: audit-records\n    callers: []\n", new: "    name: audit-records\n", want: []string{"repositories[1].callers is required"}},
		{name: "duplicate name key", old: "    name: platform-infra\n", new: "    name: platform-infra\n    name: other\n", want: []string{repository + ".name is a duplicate key"}},
		{name: "duplicate name key differing only in quotes", old: "    name: platform-infra\n", new: "    name: platform-infra\n    'name': other\n", want: []string{repository + ".name is a duplicate key"}},
		{name: "unquoted id", old: `id: "200"`, new: "id: 200", want: []string{repository + ".id must be a quoted string"}},
		{name: "block scalar id", old: "  - id: \"200\"\n", new: "  - id: |\n      200\n", want: []string{repository + ".id must be a quoted string"}},
		{name: "sequence id", old: `id: "200"`, new: "id: []", want: []string{repository + ".id must be a quoted string"}},
		{name: "zero id", old: `id: "200"`, new: `id: "0"`, want: []string{repository + ".id" + idFormat}},
		{name: "sequence name", old: "name: platform-infra", new: "name: []", want: []string{repository + ".name must be a string"}},
		{name: "mapping name", old: "name: platform-infra", new: "name: {}", want: []string{repository + ".name must be a string"}},
		{name: "integer name", old: "name: platform-infra", new: "name: 123", want: []string{repository + ".name must be a string"}},
		{name: "boolean name", old: "name: platform-infra", new: "name: true", want: []string{repository + ".name must be a string"}},
		{name: "null name", old: "name: platform-infra", new: "name: ~", want: []string{repository + ".name must be a string"}},
		{name: "empty name", old: "name: platform-infra", new: "name:", want: []string{repository + ".name must be a string"}},
		{name: "name with owner", old: "name: platform-infra", new: "name: example/platform-infra", want: []string{repository + ".name" + nameChars}},
		{name: "name of 101 characters", old: "name: platform-infra", new: "name: " + string(slices.Repeat([]byte("r"), 101)), want: []string{repository + ".name must be 1 to 100 characters"}},
		{name: "mapping callers is not a sequence", old: "callers: []", new: "callers: {}", want: []string{"repositories[1].callers must be a sequence"}},
		{name: "scalar callers is not a sequence", old: "callers: []", new: "callers: a", want: []string{"repositories[1].callers must be a sequence"}},
		{name: "duplicate id", old: `id: "201"`, new: `id: "200"`, want: []string{"repositories[1].id is a duplicate"}},
		{name: "duplicate name", old: "name: audit-records", new: "name: platform-infra", want: []string{"repositories[1].name is a duplicate"}},
		{name: "duplicate name differing only in case", old: "name: audit-records", new: "name: Platform-Infra", want: []string{"repositories[1].name is a duplicate"}},
		// 許可 Caller
		{name: "scalar allowed caller is not a mapping", old: "callers: []", new: "callers: [a]", want: []string{"repositories[1].callers[0] must be a mapping"}},
		{name: "unknown key in allowed caller", old: "        repository_name: deploy-tools\n", new: "        repository_name: deploy-tools\n        workflow: deploy\n", want: []string{caller + " has an unknown key"}},
		{name: "missing repository_id", old: "      - repository_id: \"100\"\n        repository_name", new: "      - repository_name", want: []string{caller + ".repository_id is required"}},
		{name: "missing repository_name", old: "        repository_name: deploy-tools\n", new: "", want: []string{caller + ".repository_name is required"}},
		{name: "missing permissions", old: "        permissions:\n          contents: write\n          pull_requests: read\n", new: "", want: []string{caller + ".permissions is required"}},
		{name: "unquoted repository_id", old: `repository_id: "100"`, new: "repository_id: 100", want: []string{caller + ".repository_id must be a quoted string"}},
		{name: "negative repository_id", old: `repository_id: "100"`, new: `repository_id: "-100"`, want: []string{caller + ".repository_id" + idFormat}},
		{name: "integer repository_name", old: "repository_name: deploy-tools", new: "repository_name: 1", want: []string{caller + ".repository_name must be a string"}},
		{name: "repository_name with owner", old: "repository_name: deploy-tools", new: "repository_name: example/deploy-tools", want: []string{caller + ".repository_name" + nameChars}},
		{name: "duplicate allowed caller id", old: "          pull_requests: read\n", new: "          pull_requests: read\n      - {repository_id: \"100\", repository_name: other-tools, permissions: {contents: read}}\n", want: []string{"repositories[0].callers[1].repository_id is a duplicate"}},
		{name: "duplicate allowed caller name differing only in case", old: "          pull_requests: read\n", new: "          pull_requests: read\n      - {repository_id: \"101\", repository_name: Deploy-Tools, permissions: {contents: read}}\n", want: []string{"repositories[0].callers[1].repository_name is a duplicate"}},
		// 権限の上限
		{name: "sequence permissions is not a mapping", old: "        permissions:\n          contents: write\n          pull_requests: read\n", new: "        permissions: []\n", want: []string{limit + " must be a mapping"}},
		{name: "empty permissions", old: "        permissions:\n          contents: write\n          pull_requests: read\n", new: "        permissions: {}\n", want: []string{limit + ": invalid permission limit: must have at least one permission entry"}},
		{name: "integer level is not a string", old: "contents: write", new: "contents: 1", want: []string{limit + " has a value that is not a string"}},
		{name: "boolean level is not a string", old: "contents: write", new: "contents: true", want: []string{limit + " has a value that is not a string"}},
		{name: "mapping level is not a string", old: "contents: write", new: "contents: {}", want: []string{limit + " has a value that is not a string"}},
		{name: "yes level is not in allowed permissions", old: "contents: write", new: "contents: yes", want: []string{limit + notAllowed}},
		{name: "on level is not in allowed permissions", old: "contents: write", new: "contents: on", want: []string{limit + notAllowed}},
		{name: "uppercase level", old: "contents: write", new: "contents: READ", want: []string{limit + ": invalid permission limit: entries[0] permission level must be 1 to 64 lowercase ASCII letters or '_'"}},
		{name: "malformed permission name", old: "contents: write", new: "Contents: write", want: []string{limit + ": invalid permission limit: entries[0] permission name must be 1 to 64 lowercase ASCII letters or '_'"}},
		{name: "permission name outside allowed permissions", old: "contents: write", new: "members: read", want: []string{limit + notAllowed}},
		{name: "level not defined for the permission name", old: "contents: write", new: "contents: admin", want: []string{limit + notAllowed}},
		{name: "duplicate permission name differing only in quotes", old: "pull_requests: read", new: "\"contents\": read", want: []string{limit + ": invalid permission limit: entries[1] is a duplicate permission name"}},
		{name: "integer permission name is not a string key", old: "contents: write", new: "1: write", want: []string{limit + " has a key that is not a string"}},
		// キーと YAML の機能
		{name: "sequence key is not a string", old: "    name: platform-infra\n", new: "    name: platform-infra\n    ? [a]\n    : b\n", want: []string{repository + " has a key that is not a string"}},
		{name: "integer key is not a string", old: "    name: platform-infra\n", new: "    name: platform-infra\n    1: b\n", want: []string{repository + " has a key that is not a string"}},
		{name: "null key is not a string", old: "    name: platform-infra\n", new: "    name: platform-infra\n    ~: b\n", want: []string{repository + " has a key that is not a string"}},
		{name: "merge key", old: "    name: platform-infra\n", new: "    name: platform-infra\n    <<: {}\n", want: []string{"line 5: must not have a merge key"}},
		{name: "quoted merge key is an unknown key", old: "    name: platform-infra\n", new: "    name: platform-infra\n    \"<<\": {}\n", want: []string{repository + " has an unknown key"}},
		{name: "anchor on a value", old: "name: platform-infra", new: "name: &n platform-infra", want: []string{"line 4: must not have an anchor"}},
		{name: "anchor on a key", old: "    name: platform-infra", new: "    &k name: platform-infra", want: []string{"line 4: must not have an anchor"}},
		{name: "anchor on a mapping", old: "  - id: \"201\"", new: "  - &r\n    id: \"201\"", want: []string{"line 12: must not have an anchor"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := replaceOnce(t, validDocument, tt.old, tt.new)
			if _, conditions := read(t, document); !slices.Equal(conditions, tt.want) {
				t.Errorf("conditions = %q, want %q\ndocument:\n%s", conditions, tt.want, document)
			}
		})
	}
}

// エイリアスは展開せず、その位置の種類の誤りを重ねて報告しない
func TestReadDocument_Alias(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     []string
	}{
		{
			name:     "alias of callers",
			document: replaceOnce(t, replaceOnce(t, validDocument, "    callers:\n", "    callers: &c\n"), "callers: []", "callers: *c"),
			want:     []string{"line 5: must not have an anchor", "line 14: must not be an alias"},
		},
		{
			name:     "alias of a level",
			document: replaceOnce(t, replaceOnce(t, validDocument, "contents: write", "contents: &l write"), "pull_requests: read", "pull_requests: *l"),
			want:     []string{"line 9: must not have an anchor", "line 10: must not be an alias"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, conditions := read(t, tt.document); !slices.Equal(conditions, tt.want) {
				t.Errorf("conditions = %q, want %q", conditions, tt.want)
			}
		})
	}
}

// 権限の上限は書かれた順の組の位置で誤りを示すため、YAML として読めない組があれば作らず、残りの組の誤りを報告しない
func TestReadDocument_LimitIsNotBuiltFromUnreadablePairs(t *testing.T) {
	const limit = "repositories[0].callers[0].permissions"
	tests := []struct {
		name  string
		entry string
		want  []string
	}{
		{name: "key that is not a string", entry: "1: write", want: []string{limit + " has a key that is not a string"}},
		{name: "merge key", entry: "<<: {}", want: []string{"line 9: must not have a merge key"}},
		{name: "alias key", entry: "contents: &k write\n          *k : read", want: []string{"line 10: must not be an alias", "line 9: must not have an anchor"}},
		{name: "alias value", entry: "contents: &l write\n          issues: *l", want: []string{"line 9: must not have an anchor", "line 10: must not be an alias"}},
		{name: "value that is not a string", entry: "contents: 1", want: []string{limit + " has a value that is not a string"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := replaceOnce(t, replaceOnce(t, validDocument, "contents: write", tt.entry), "pull_requests: read", "pull_requests: READ")
			if _, conditions := read(t, document); !slices.Equal(conditions, tt.want) {
				t.Errorf("conditions = %q, want %q", conditions, tt.want)
			}
		})
	}
}

func TestReadDocument_ReportsTheSameConditionOnce(t *testing.T) {
	document := replaceOnce(t, validDocument, "    name: platform-infra\n", "    name: platform-infra\n    owner: a\n    team: b\n")
	want := []string{"repositories[0] has an unknown key"}
	if _, conditions := read(t, document); !slices.Equal(conditions, want) {
		t.Errorf("conditions = %q, want %q", conditions, want)
	}
}

func TestReadDocument_ReportsEveryCondition(t *testing.T) {
	document := replaceOnce(t, replaceOnce(t, validDocument, `id: "201"`, `id: "0201"`), "        repository_name: deploy-tools\n", "        repository_name: deploy-tools\n        workflow: deploy\n")
	want := []string{
		"repositories[0].callers[0] has an unknown key",
		"repositories[1].id must be a positive decimal integer without a sign or leading zeros that fits in int64",
	}
	if _, conditions := read(t, document); !slices.Equal(conditions, want) {
		t.Errorf("conditions = %q, want %q", conditions, want)
	}
}

// 構造の誤りの中はたどらないため、誤った位置の下にある値の誤りは報告しない
func TestReadDocument_DoesNotDescendIntoInvalidValues(t *testing.T) {
	document := "repositories: [a]\nunknown: {id: 0, name: &x a}\n"
	want := []string{"document has an unknown key", "repositories[0] must be a mapping"}
	if _, conditions := read(t, document); !slices.Equal(conditions, want) {
		t.Errorf("conditions = %q, want %q", conditions, want)
	}
}

// read は、文字列の検査と文書の数を通る文書の構文木を読み、読み取りの結果と条件を返す。
func read(t *testing.T, document string) ([]protectedRepository, []string) {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(document), &root); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	var c conditions
	got := readDocument(&root, &c)
	return got, c.list
}
