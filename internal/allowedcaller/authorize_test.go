package allowedcaller

import (
	"testing"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
)

// authorizeDocument は、権限の上限の異なる 3 つの許可 Caller を並べる。
const authorizeDocument = `callers:
  - repository_id: "100"
    repository_name: deploy-tools
    permissions:
      contents: write
      pull_requests: read
      repository_projects: admin
  - repository_id: "300"
    repository_name: release-tools
    permissions:
      contents: read
  - repository_id: "400"
    repository_name: metadata-tools
    permissions:
      contents: read
      metadata: read
`

//trace:checks AC-AUTHZ-011
func TestSet_Authorize(t *testing.T) {
	tests := []struct {
		name        string
		callerID    int64
		callerName  string
		levels      map[string]string
		wantVerdict Verdict
		wantIndex   int
	}{
		{name: "same level as the permission limit is allowed", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"contents": "write", "pull_requests": "read"}, wantVerdict: Allowed, wantIndex: 0},
		{name: "allowed caller name differing only in case is allowed", callerID: 100, callerName: "Deploy-Tools", levels: map[string]string{"contents": "read"}, wantVerdict: Allowed, wantIndex: 0},
		{name: "later allowed caller is allowed with its index", callerID: 300, callerName: "release-tools", levels: map[string]string{"contents": "read"}, wantVerdict: Allowed, wantIndex: 1},
		{name: "explicit implied permission within the permission limit is allowed", callerID: 400, callerName: "metadata-tools", levels: map[string]string{"contents": "read", "metadata": "read"}, wantVerdict: Allowed, wantIndex: 2},
		{name: "higher level than the permission limit exceeds it", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"pull_requests": "write"}, wantVerdict: ExceedsPermissionLimit, wantIndex: 0},
		{name: "level outside allowed permissions exceeds the permission limit", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"contents": "admin"}, wantVerdict: ExceedsPermissionLimit, wantIndex: 0},
		{name: "id only match is a name mismatch", callerID: 100, callerName: "renamed-tools", levels: map[string]string{"contents": "read"}, wantVerdict: NameMismatch, wantIndex: 0},
		{name: "name only match is an id mismatch", callerID: 999, callerName: "release-tools", levels: map[string]string{"contents": "read"}, wantVerdict: IDMismatch, wantIndex: 1},
		{name: "id only and name only matches are a name mismatch at the id match", callerID: 100, callerName: "release-tools", levels: map[string]string{"contents": "read"}, wantVerdict: NameMismatch, wantIndex: 0},
		{name: "caller matching neither has no matching allowed caller", callerID: 999, callerName: "other", levels: map[string]string{"contents": "read"}, wantVerdict: NoMatchingAllowedCaller, wantIndex: -1},
	}
	set := mustRead(t, authorizeDocument)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict, k := set.Authorize(mustRepositoryID(t, tt.callerID), tt.callerName, mustNewPermissions(t, tt.levels))
			if verdict != tt.wantVerdict || k != tt.wantIndex {
				t.Errorf("Authorize = %v at %d, want %v at %d", verdict, k, tt.wantVerdict, tt.wantIndex)
			}
		})
	}
}

func TestSet_Authorize_ZeroValues(t *testing.T) {
	contentsRead := mustNewPermissions(t, map[string]string{"contents": "read"})
	tests := []struct {
		name        string
		set         Set
		callerID    githubid.RepositoryID
		permissions permission.Permissions
		wantVerdict Verdict
		wantIndex   int
	}{
		{name: "empty set has no allowed callers", set: mustRead(t, "callers: []\n"), callerID: mustRepositoryID(t, 100), permissions: contentsRead, wantVerdict: NoAllowedCallers, wantIndex: -1},
		{name: "zero set has no allowed callers", set: Set{}, callerID: mustRepositoryID(t, 100), permissions: contentsRead, wantVerdict: NoAllowedCallers, wantIndex: -1},
		{name: "zero caller repository id matches no id", set: mustRead(t, authorizeDocument), permissions: contentsRead, wantVerdict: IDMismatch, wantIndex: 0},
		{name: "zero permissions exceed the permission limit", set: mustRead(t, authorizeDocument), callerID: mustRepositoryID(t, 100), wantVerdict: ExceedsPermissionLimit, wantIndex: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict, k := tt.set.Authorize(tt.callerID, "deploy-tools", tt.permissions)
			if verdict != tt.wantVerdict || k != tt.wantIndex {
				t.Errorf("Authorize = %v at %d, want %v at %d", verdict, k, tt.wantVerdict, tt.wantIndex)
			}
		})
	}
}
