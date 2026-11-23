package allowedcaller

import (
	"slices"
	"testing"
)

func TestRead(t *testing.T) {
	set := mustRead(t, `callers:
  - {repository_id: "100", repository_name: deploy-tools, permissions: {contents: write}}
  - {repository_id: "300", repository_name: release-tools, permissions: {issues: read}}
`)
	if verdict, k := set.Authorize(mustRepositoryID(t, 100), "deploy-tools", mustNewPermissions(t, map[string]string{"contents": "write"})); verdict != Allowed || k != 0 {
		t.Errorf("verdict for callers[0] = %v at %d, want Allowed at 0", verdict, k)
	}
	if verdict, k := set.Authorize(mustRepositoryID(t, 300), "release-tools", mustNewPermissions(t, map[string]string{"issues": "read"})); verdict != Allowed || k != 1 {
		t.Errorf("verdict for callers[1] = %v at %d, want Allowed at 1", verdict, k)
	}
}

//trace:checks AC-AUTHZ-013
func TestRead_Invalid(t *testing.T) {
	const idFormat = " must be a positive decimal integer without a sign or leading zeros that fits in int64"
	tests := []struct {
		name     string
		document string
		want     []string
	}{
		{name: "mapping is not a sequence", document: "callers: {}\n", want: []string{"callers must be a sequence"}},
		{name: "element that is not a mapping is skipped and later elements keep their positions", document: "callers: [a, {repository_id: \"0\", repository_name: b, permissions: {contents: read}}]\n", want: []string{"callers[0] must be a mapping", "callers[1].repository_id" + idFormat}},
		{name: "unknown key", document: "callers: [{repository_id: \"1\", repository_name: a, permissions: {contents: read}, workflow: deploy}]\n", want: []string{"callers[0] has an unknown key"}},
		{name: "missing repository_id", document: "callers: [{repository_name: a, permissions: {contents: read}}]\n", want: []string{"callers[0].repository_id is required"}},
		{name: "missing repository_name", document: "callers: [{repository_id: \"1\", permissions: {contents: read}}]\n", want: []string{"callers[0].repository_name is required"}},
		{name: "missing permissions", document: "callers: [{repository_id: \"1\", repository_name: a}]\n", want: []string{"callers[0].permissions is required"}},
		{name: "empty permissions", document: "callers: [{repository_id: \"1\", repository_name: a, permissions: {}}]\n", want: []string{"callers[0].permissions: invalid permission limit: must have at least one permission entry"}},
		{name: "duplicate id", document: "callers: [{repository_id: \"1\", repository_name: a, permissions: {contents: read}}, {repository_id: \"1\", repository_name: b, permissions: {contents: read}}]\n", want: []string{"callers[1].repository_id is a duplicate"}},
		{name: "duplicate name differing only in case", document: "callers: [{repository_id: \"1\", repository_name: deploy-tools, permissions: {contents: read}}, {repository_id: \"2\", repository_name: Deploy-Tools, permissions: {contents: read}}]\n", want: []string{"callers[1].repository_name is a duplicate"}},
		{name: "unreadable ids are not duplicates", document: "callers: [{repository_id: \"0\", repository_name: a, permissions: {contents: read}}, {repository_id: \"0\", repository_name: b, permissions: {contents: read}}]\n", want: []string{"callers[0].repository_id" + idFormat, "callers[1].repository_id" + idFormat}},
		{name: "unreadable names are not duplicates", document: "callers: [{repository_id: \"1\", repository_name: a/b, permissions: {contents: read}}, {repository_id: \"2\", repository_name: a/b, permissions: {contents: read}}]\n", want: []string{"callers[0].repository_name must consist of ASCII letters, digits, '-', '_', or '.'", "callers[1].repository_name must consist of ASCII letters, digits, '-', '_', or '.'"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, conditions := read(t, tt.document); !slices.Equal(conditions, tt.want) {
				t.Errorf("conditions = %q, want %q", conditions, tt.want)
			}
		})
	}
}
