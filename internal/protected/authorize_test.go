package protected

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/repository"
)

// authorizeDocument は、許可 Caller が 2 つの保護対象、許可 Caller のない保護対象、別の許可 Caller だけを持つ保護対象を並べる。
const authorizeDocument = `repositories:
  - id: "200"
    name: platform-infra
    callers:
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
  - id: "201"
    name: audit-records
    callers: []
  - id: "202"
    name: shared-config
    callers:
      - repository_id: "300"
        repository_name: release-tools
        permissions:
          contents: read
`

//trace:checks AC-AUTHZ-011
func TestRepositories_Authorize(t *testing.T) {
	const (
		platform = "example-org/platform-infra"
		audit    = "example-org/audit-records"
		shared   = "example-org/shared-config"
		app      = "example-org/app"
	)
	tests := []struct {
		name         string
		document     string
		callerID     int64
		callerName   string
		repositories []string
		levels       map[string]string
		wantErr      error
		wantMessage  string
	}{
		{name: "list without protected repositories accepts any request", document: "repositories: []\n", callerID: 999, callerName: "other", repositories: []string{platform}, levels: map[string]string{"contents": "write"}},
		{name: "request without protected repositories is accepted", callerID: 999, callerName: "other", repositories: []string{app}, levels: map[string]string{"contents": "write"}},
		{name: "same level as the permission limit is accepted", callerID: 100, callerName: "deploy-tools", repositories: []string{platform}, levels: map[string]string{"contents": "write", "pull_requests": "read"}},
		{name: "read under write is accepted", callerID: 100, callerName: "deploy-tools", repositories: []string{platform}, levels: map[string]string{"contents": "read"}},
		{name: "write and read under admin are accepted", callerID: 100, callerName: "deploy-tools", repositories: []string{platform}, levels: map[string]string{"repository_projects": "write", "contents": "read"}},
		{name: "allowed caller name differing only in case is accepted", callerID: 100, callerName: "Deploy-Tools", repositories: []string{platform}, levels: map[string]string{"contents": "read"}},
		{name: "protected and unprotected targets allowed for the caller are accepted", callerID: 100, callerName: "deploy-tools", repositories: []string{app, platform}, levels: map[string]string{"contents": "read"}},
		{name: "two protected repositories allowing the caller are accepted", callerID: 300, callerName: "release-tools", repositories: []string{platform, shared}, levels: map[string]string{"contents": "read"}},
		{
			name: "higher level than the permission limit is denied", callerID: 100, callerName: "deploy-tools", repositories: []string{platform}, levels: map[string]string{"pull_requests": "write"},
			wantErr: errExceedsPermissionLimit, wantMessage: "repositories[0] matches protected repositories[0]: permissions are not within the permission limit of callers[0]",
		},
		{
			name: "permission name outside the permission limit is denied", callerID: 100, callerName: "deploy-tools", repositories: []string{platform}, levels: map[string]string{"issues": "read"},
			wantErr: errExceedsPermissionLimit, wantMessage: "repositories[0] matches protected repositories[0]: permissions are not within the permission limit of callers[0]",
		},
		{
			name: "permissions only partly within the permission limit are denied", callerID: 300, callerName: "release-tools", repositories: []string{platform}, levels: map[string]string{"contents": "read", "pull_requests": "read"},
			wantErr: errExceedsPermissionLimit, wantMessage: "repositories[0] matches protected repositories[0]: permissions are not within the permission limit of callers[1]",
		},
		{
			name: "explicit implied permission outside the permission limit is denied", callerID: 100, callerName: "deploy-tools", repositories: []string{platform}, levels: map[string]string{"contents": "read", "metadata": "read"},
			wantErr: errExceedsPermissionLimit, wantMessage: "repositories[0] matches protected repositories[0]: permissions are not within the permission limit of callers[0]",
		},
		{
			name: "caller matching no allowed caller is denied", callerID: 999, callerName: "other", repositories: []string{platform}, levels: map[string]string{"contents": "read"},
			wantErr: errNoMatchingAllowedCaller, wantMessage: "repositories[0] matches protected repositories[0]: no allowed caller matches both the caller repository id and name",
		},
		{
			name: "caller matching only the id of an allowed caller is denied", callerID: 100, callerName: "renamed-tools", repositories: []string{platform}, levels: map[string]string{"contents": "read"},
			wantErr: errAllowedCallerNameMismatch, wantMessage: "repositories[0] matches protected repositories[0]: callers[0] matches the caller repository id but not the name",
		},
		{
			name: "caller matching only the name of an allowed caller is denied", callerID: 999, callerName: "deploy-tools", repositories: []string{platform}, levels: map[string]string{"contents": "read"},
			wantErr: errAllowedCallerIDMismatch, wantMessage: "repositories[0] matches protected repositories[0]: callers[0] matches the caller repository name but not the id",
		},
		{
			name: "caller matching the id of one allowed caller and the name of another is denied by the id match", callerID: 100, callerName: "release-tools", repositories: []string{platform}, levels: map[string]string{"contents": "read"},
			wantErr: errAllowedCallerNameMismatch, wantMessage: "repositories[0] matches protected repositories[0]: callers[0] matches the caller repository id but not the name",
		},
		{
			name: "caller name outside the repository name rule is denied without another error", callerID: 100, callerName: "deploy tools!" + strings.Repeat("x", 100), repositories: []string{platform}, levels: map[string]string{"contents": "read"},
			wantErr: errAllowedCallerNameMismatch, wantMessage: "repositories[0] matches protected repositories[0]: callers[0] matches the caller repository id but not the name",
		},
		{
			name: "protected repository without allowed callers is denied", callerID: 100, callerName: "deploy-tools", repositories: []string{audit}, levels: map[string]string{"contents": "read"},
			wantErr: errNoAllowedCallers, wantMessage: "repositories[0] matches protected repositories[1]: no allowed callers are listed",
		},
		{
			name: "protected target mixed with unprotected targets is denied as a whole", callerID: 999, callerName: "other", repositories: []string{app, platform}, levels: map[string]string{"contents": "read"},
			wantErr: errNoMatchingAllowedCaller, wantMessage: "repositories[1] matches protected repositories[0]: no allowed caller matches both the caller repository id and name",
		},
		{
			name: "only one of two protected repositories allowing the caller is denied", callerID: 100, callerName: "deploy-tools", repositories: []string{platform, shared}, levels: map[string]string{"contents": "read"},
			wantErr: errNoMatchingAllowedCaller, wantMessage: "repositories[1] matches protected repositories[2]: no allowed caller matches both the caller repository id and name",
		},
		{
			name: "target name differing only in case matches the protected repository", callerID: 999, callerName: "other", repositories: []string{"example-org/PLATFORM-INFRA"}, levels: map[string]string{"contents": "read"},
			wantErr: errNoMatchingAllowedCaller, wantMessage: "repositories[0] matches protected repositories[0]: no allowed caller matches both the caller repository id and name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := tt.document
			if document == "" {
				document = authorizeDocument
			}
			requested := make([]repository.FullName, 0, len(tt.repositories))
			for _, s := range tt.repositories {
				requested = append(requested, mustParseFullName(t, s))
			}
			err := mustParse(t, document).Authorize(mustRepositoryID(t, tt.callerID), tt.callerName, requested, mustNewPermissions(t, tt.levels))
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("Authorize returned error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) || err.Error() != tt.wantMessage {
				t.Errorf("Authorize error = %v, want %q wrapping %v", err, tt.wantMessage, tt.wantErr)
			}
		})
	}
}

func TestRepositories_AuthorizeZeroValue(t *testing.T) {
	err := Repositories{}.Authorize(mustRepositoryID(t, 1), "other", []repository.FullName{mustParseFullName(t, "example-org/app")}, mustNewPermissions(t, map[string]string{"contents": "read"}))
	if !errors.Is(err, errUninitialized) {
		t.Errorf("Authorize error = %v, want %v", err, errUninitialized)
	}
}
